package state

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"time"

	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/watchdog"
	tele_api "github.com/AlexTransit/vender/tele"
)

func (g *Global) CheckMenuExecution() {
	// FIXME aAlexM переделать проверку сценария меню
	// сейчас заполняю по максимуму склад, что бы проверить сченарий через валидатор
	for i := range g.Inventory.Stocks {
		g.Inventory.Stocks[i].Set(math.MaxFloat32)
	}
	for _, v := range g.Config.Engine.Menu.Items {
		if e := v.Doer.Validate(); e != nil {
			g.Log.Errorf("scenario menu code:%s error (%v)", v.Code, e)
		}
	}
	g.Inventory.InventoryLoad()
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
	if g.UI().GetUiState() != uint32(types.StateFrontSelect) {
		watchdog.DevicesInitializationRequired()
	}
	g.VmcStopWOInitRequared(ctx)
}

func (g *Global) VmcStopWOInitRequared(ctx context.Context) {
	watchdog.Disable()
	g.Log.Infof("--- event vmc stop ---")
	go func() {
		time.Sleep(10 * time.Second)
		g.Log.Infof("--- vmc timeout EXIT ---")
		os.Exit(0)
	}()
	g.UI().CreateEvent(types.EventStop)
	time.Sleep(2 * time.Second)
	g.Stop()
	g.Tele.Close()
	g.Alive.Wait()
	g.Log.Infof("--- vmc stop ---")
	os.Exit(0)
}

func (g *Global) prepareInventory() {
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
		s.XXX_Ingredient = ""
		g.Inventory.Stocks = append(g.Inventory.Stocks, s)
	}
	g.Config.Inventory.XXX_Stocks = nil
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

// func (g *Global) Error(err error, args ...interface{}) {
func (g *Global) Error(err error) {
	if err == nil {
		return
	}
	g.Log.Error(err)
}

// func (g *Global) Fatal(err error, args ...interface{}) {
func (g *Global) Fatal(err error) {
	if err != nil {
		// FIXME alexm
		sound.PlayFile("broken.mp3")
		// g.Error(err, args...)
		g.Error(err)
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

// send broken message
func (g *Global) SendBroken(errorMessage ...string) {
	rm := tele_api.FromRoboMessage{
		State: tele_api.State_Broken,
	}
	if len(errorMessage[0]) != 0 {
		rm.Err = &tele_api.Err{
			Code:    0,
			Message: errorMessage[0],
		}
	}
	// if the order is not completed, the order is canceled
	if g.Config.User.PaymenId != 0 {
		rm.Order = g.OrderToMessage()
		rm.Order.OrderStatus = tele_api.OrderStatus_orderError
	}
	g.Tele.RoboSend(&rm)
}

func (g *Global) SendCooking() {
	rm := tele_api.FromRoboMessage{State: tele_api.State_Process}
	if g.Config.User.PaymenId != 0 {
		rm.Order = g.OrderToMessage()
		rm.Order.OrderStatus = tele_api.OrderStatus_executionStart
	}

	g.Tele.RoboSend(&rm)
}

func (g *Global) OrderToMessage() *tele_api.Order {
	o := &tele_api.Order{
		MenuCode:      g.Config.User.SelectedItem.Code,
		Amount:        uint32(g.Config.User.DirtyMoney),
		PaymentMethod: g.Config.User.PaymentMethod,
		OwnerInt:      g.Config.User.PaymenId,
		OwnerType:     g.Config.User.PaymentType,
	}
	return o
}
