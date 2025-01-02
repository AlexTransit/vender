package state

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/watchdog"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
)

// func (g *Global) CheckMenuExecution(ctx context.Context) (err error) {
// 	ch := g.Inventory.DisableCheckInStock()
// 	for _, v := range g.Config.Engine.Menu.Items {
// 		e := v.Doer.Validate()
// 		if e != nil {
// 			s := strings.Split(e.Error(), "\n")
// 			g.Log.Errorf("menu code:%s not valid (%s) in scenario(%s)", v.Code, s[0], v.Scenario)
// 		}
// 	}
// 	g.Inventory.EnableCheckInStock(&ch)
// 	return err
// }

func VmcLock(ctx context.Context) {
	g := GetGlobal(ctx)
	g.Log.Info("Vmc Locked")
	config_global.VMC.User.Lock = true
	config_global.VMC.User.KeyboardReadEnable = false
	if config_global.VMC.User.UiState == uint32(types.StateFrontSelect) || config_global.VMC.User.UiState == uint32(types.StatePrepare) {
		g.LockCh <- struct{}{}
	}
}

func VmcUnLock(ctx context.Context) {
	g := GetGlobal(ctx)
	g.Log.Info("Vmc UnLocked")
	config_global.VMC.User.Lock = false
	config_global.VMC.KeyboardReader(true)
	if config_global.VMC.User.UiState == uint32(types.StateFrontLock) {
		g.LockCh <- struct{}{}
	}
}

func (g *Global) UpgradeVender() {
	go func() {
		if err := g.RunBashSript(g.Config.UpgradeScript); err != nil {
			g.Log.Errorf("upgrade err(%v)", err)
			return
		}
		config_global.VMC.Engine.NeedRestart = true
	}()
}

func (g *Global) VmcStop(ctx context.Context) {
	if config_global.VMC.User.UiState != uint32(types.StateFrontSelect) {
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
	// put overrided stock to stock
	for _, v := range g.Config.Inventory.XXX_Ingredient {
		g.Inventory.Ingredient = append(g.Inventory.Ingredient, v)
	}
	g.Config.Inventory.XXX_Ingredient = nil

	for _, v := range g.Config.Inventory.XXX_Stocks {
		s := inventory.Stock{}
		s = v
		s.Log = g.Log
		s.Ingredient = g.Inventory.GetIngredientByName(v.XXX_Ingredient)
		g.Inventory.Stocks = append(g.Inventory.Stocks, s)
	}
	g.Config.Inventory.XXX_Stocks = nil

	if err := g.Inventory.Init(ctx, g.Engine); err != nil {
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
	config_global.VMC.User.QrText = t
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
	if !config_global.VMC.User.Lock {
		config_global.VMC.User.Lock = true
		g.Log.Infof("--- client activity begin ---")
	}
	g.Tele.RoboSendState(tele_api.State_Client)
}

func (g *Global) ClientEnd(ctx context.Context) {
	config_global.VMC.KeyboardReader(true)
	if config_global.VMC.User.Lock {
		config_global.VMC.User.Lock = false
		g.Log.Infof("--- client activity end ---")
	}
}

func (g *Global) MustInit(ctx context.Context, cfg *config_global.Config) {
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

func (g *Global) initDisplay() {
	d, err := g.Display()
	if d == nil || err != nil {
		g.Config.Hardware.Display.Framebuffer = ""
	}
}
