package state

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/watchdog"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
)

func (g *Global) CheckMenuExecution(ctx context.Context) (err error) {
	ch := g.Inventory.DisableCheckInStock()
	for _, v := range g.Config.Engine.Menu.Items {
		e := v.Doer.Validate()
		if e != nil {
			s := strings.Split(e.Error(), "\n")
			g.Log.Errorf("menu code:%s not valid (%s) in scenario(%s)", v.Code, s[0], v.Scenario)
		}
	}
	g.Inventory.EnableCheckInStock(&ch)
	return err
}

func VmcLock(ctx context.Context) {
	g := GetGlobal(ctx)
	g.Log.Info("Vmc Locked")
	types.VMC.Lock = true
	types.VMC.EvendKeyboardInput(false)
	if types.VMC.UiState == uint32(types.StateFrontSelect) || types.VMC.UiState == uint32(types.StatePrepare) {
		g.LockCh <- struct{}{}
	}
}

func VmcUnLock(ctx context.Context) {
	g := GetGlobal(ctx)
	g.Log.Info("Vmc UnLocked")
	types.VMC.Lock = false
	types.VMC.EvendKeyboardInput(true)
	if types.VMC.UiState == uint32(types.StateFrontLock) {
		g.LockCh <- struct{}{}
	}
}

func (g *Global) UpgradeVender() {
	go func() {
		if err := g.RunBashSript(g.Config.UpgradeScript); err != nil {
			g.Log.Errorf("upgrade err(%v)", err)
			return
		}
		types.VMC.NeedRestart = true
	}()
}

func (g *Global) VmcStop(ctx context.Context) {
	if types.VMC.UiState != uint32(types.StateFrontSelect) {
		watchdog.DevicesInitializationRequired()
	}
	g.VmcStopWOInitRequared(ctx)
}

func (g *Global) VmcStopWOInitRequared(ctx context.Context) {
	watchdog.Disable()
	g.Log.Infof("--- event vmc stop ---")
	go func() {
		time.Sleep(3 * time.Second)
		g.Log.Infof("--- vmc timeout EXIT ---")
		os.Exit(0)
	}()
	g.LockCh <- struct{}{}
	_ = g.Engine.ExecList(ctx, "on_broken", g.Config.Engine.OnBroken)
	g.Tele.Close()
	time.Sleep(2 * time.Second)
	g.Log.Infof("--- vmc stop ---")
	g.Stop()
	g.Alive.Done()
	os.Exit(0)
}

func (g *Global) initInventory(ctx context.Context) error {
	// TODO ctx should be enough
	if err := g.Inventory.Init(ctx, &g.Config.Engine.Inventory, g.Engine, g.Config.Persist.Root); err != nil {
		return err
	}
	g.Inventory.InventoryLoad()
	return nil
}

func (g *Global) RunBashSript(script string) (err error) {
	if script == "" {
		return nil
	}
	cmd := exec.Command("/usr/bin/bash", "-c", script)
	stdout, e := cmd.Output()
	if e == nil {
		return nil
	}
	return fmt.Errorf("script(%s) stdout(%s) error(%s)", script, stdout, cmd.Stderr)
}

func (g *Global) initDisplay() error {
	d, err := g.Display()
	if d != nil {
		types.VMC.HW.Display.GdisplayValid = true
	}
	return err
}

func (g *Global) ShowQR(t string) {
	display, err := g.Display()
	if err != nil {
		g.Log.Error(err, "display")
		return
	}
	if display == nil {
		g.Log.Error("display is not configured")
		return
	}
	g.Log.Infof("show QR:'%v'", t)
	err = display.QR(t, true, 2)
	if err != nil {
		g.Log.Error(err, "QR show error")
	}
	types.VMC.HW.Display.Gdisplay = t
}

func (g *Global) Stop() {
	g.Alive.Stop()
}

func (g *Global) StopWait(timeout time.Duration) bool {
	g.Alive.Stop()
	select {
	case <-g.Alive.WaitChan():
		return true
	case <-time.After(timeout):
		return false
	}
}

func (g *Global) ClientBegin(ctx context.Context) {
	_ = g.Engine.ExecList(ctx, "client-light", []string{"evend.cup.light_on"})
	if !types.VMC.Lock {
		// g.TimerUIStop <- struct{}{}
		types.VMC.Lock = true
		types.VMC.Client.WorkTime = time.Now()
		g.Log.Infof("--- client activity begin ---")
	}
	g.Tele.RoboSendState(tele_api.State_Client)
}

func (g *Global) ClientEnd(ctx context.Context) {
	types.VMC.EvendKeyboardInput(true)
	if types.VMC.Lock {
		types.VMC.Lock = false
		types.VMC.Client.WorkTime = time.Now()
		g.Log.Infof("--- client activity end ---")
	}
}

func (g *Global) MustInit(ctx context.Context, cfg *Config) {
	err := g.Init(ctx, g.Config)
	if err != nil {
		g.Fatal(err)
	}
}

func (g *Global) Error(err error, args ...interface{}) {
	if err != nil {
		if len(args) != 0 {
			msg := args[0].(string)
			args = args[1:]
			err = errors.Annotatef(err, msg, args...)
		}
		// g.Tele.Error(err)
		// эта бабуйня еще и в телеметрию отсылает
		g.Log.Error(err)
	}
}

func (g *Global) Fatal(err error, args ...interface{}) {
	if err != nil {
		// FIXME alexm
		sound.PlayFile("broken.mp3")
		g.Error(err, args...)
		g.StopWait(5 * time.Second)
		g.Log.Fatal(err)
		os.Exit(1)
	}
}
