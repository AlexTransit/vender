package state

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"time"

	"github.com/AlexTransit/vender/currency"
	config_global "github.com/AlexTransit/vender/internal/config"
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
		c := v.Doer.Calculation()
		cost := currency.Amount(int(math.Round(c * 100)))
		if cost > 0 && cost >= v.Price {
			g.Log.Errorf("!!!!best price code:%s price:%v cost:%v", v.Code, v.Price.Format100I(), cost.Format100I())
		}
		// g.Log.Infof("menu - code:%s price:%v cost:%v", v.Code, v.Price, cost)
	}
	g.Inventory.InventoryLoad()
}

func (g *Global) ListMenuPriceCost() {
	g.Log.Infof("code;price;cost")
	for _, v := range g.Config.Engine.Menu.Items {
		c := v.Doer.Calculation()
		cost := currency.Amount(int(math.Round(c * 100)))
		g.Log.Infof("%s;%v;%v", v.Code, v.Price.Format100I(), cost.Format100I())
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
	a := g.XXX_uier.Load()
	if a == nil || g.UI().GetUiState() != uint32(types.StateFrontSelect) {
		watchdog.DevicesInitializationRequired()
	}
	g.VmcStopWOInitRequared(ctx)
}

func (g *Global) VmcStopWOInitRequared(ctx context.Context) {
	watchdog.Disable()
	g.TeleCancelOrder(tele_api.State_Shutdown)
	g.Log.Infof("--- event vmc stop ---")
	go func() {
		time.Sleep(10 * time.Second)
		g.Log.Infof("--- vmc timeout EXIT ---")
		os.Exit(0)
	}()
	g.Engine.ExecList(ctx, "on_shutdown", g.Config.Engine.OnShutdown)
	// g.UI().CreateEvent(types.EventStop)
	time.Sleep(2 * time.Second)
	g.Stop()
	g.Tele.Close()
	g.Alive.Wait()
	g.Log.Infof("--- vmc stop ---")
	os.Exit(0)
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
		// g.Engine.ExecList(context.Background(), "on_broken", g.Config.Engine.OnBroken)
		g.Engine.Exec(context.Background(), g.Engine.Resolve("sound(broken.mp3)"))
		g.Error(err)
		time.Sleep(2 * time.Second)
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

// send state, error and cancel uncoplete order message
func (g *Global) TeleCancelOrder(s tele_api.State) {
	rm := tele_api.FromRoboMessage{
		State: s,
	}
	// if the order is not completed, the order is canceled
	if g.Config.User.PaymenId != 0 {
		rm.Order = g.OrderToMessage()
		if g.Config.User.DirtyMoney != 0 {
			rm.Order.OrderStatus = tele_api.OrderStatus_orderError // return cashless money
		} else {
			// order full maked and not completed.
			rm.Order.OrderStatus = tele_api.OrderStatus_complete
			g.GlobalError += fmt.Sprintf("set complete order in cancel order point. paymentId:%d dirty money=0", g.Config.User.PaymenId)
		}
	}
	if g.GlobalError != "" {
		rm.Err = &tele_api.Err{
			Code:    0,
			Message: g.GetGlobalErr(),
		}
	}
	if rm.Order != nil || rm.Err != nil {
		g.Tele.RoboSend(&rm)
	} else {
		g.Tele.RoboSendState(s)
	}
}

func (g *Global) TeleCancelQr(s tele_api.State) {
	rm := tele_api.FromRoboMessage{
		State: s,
	}
	if g.Config.User.PaymenId != 0 {
		rm.Order = g.OrderToMessage()
		rm.Order.OrderStatus = tele_api.OrderStatus_cancel
		g.Tele.RoboSend(&rm)
	} else {
		g.Tele.RoboSendState(s)
	}
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
		Amount:        uint32(config_global.VMC.User.SelectedItem.Price),
		PaymentMethod: g.Config.User.PaymentMethod,
		OwnerInt:      g.Config.User.PaymenId,
		OwnerType:     g.Config.User.PaymentType,
	}
	return o
}

// func (g *Global) Broken(ctx context.Context) {
// 	watchdog.SetBroken()
// 	g.TeleCancelOrder(tele_api.State_Broken)
// 	g.Display()
// 	display := g.MustTextDisplay()
// 	// FIXME alexm
// 	display.SetLine(1, "ABTOMAT")
// 	display.SetLine(2, "HE ABTOMAT :(")
// 	g.RunBashSript(g.Config.ScriptIfBroken)
// 	if errs := g.Engine.ExecList(ctx, "on_broken", g.Config.Engine.OnBroken); len(errs) != 0 {
// 		g.Log.Error(errors.ErrorStack(errors.Annotate(helpers.FoldErrors(errs), "on_broken")))
// 	}
// 	// 	moneysys := money.GetGlobal(ctx)
// 	// 	_ = moneysys.SetAcceptMax(ctx, 0)
// 	// }

// 	// FIXME alexm
// 	// g.Engine.Exec(ctx, g.Engine.Resolve("sound(broken.mp3)"))
// 	// sound.PlayFile("broken.mp3")
// 	// g.Snd.PlayFile("broken.mp3")
// 	// g.Stop()
// 	// g.Tele.Close()

// 	go func() {
// 		for {
// 			watchdog.Refresh()
// 			time.Sleep(time.Duration(g.Config.UI_config.Front.ResetTimeoutSec / 2))
// 		}
// 	}()
// 	// e := ui.wait(time.Second)
// 	// // TODO receive tele command to reboot or change state
// 	// if e.Kind == types.EventService {
// 	// 	return types.StateServiceBegin
// 	// }

// 	// srcServiceKey, _ := input.NewDevInputEventSource(g.Config.Hardware.Input.ServiceKey)
// 	// time.Sleep(2 * time.Minute)
// 	// e, err := srcServiceKey.Read() // wait press service key
// 	// fmt.Printf("\033[41m %v \033[0m\n", e)
// 	// fmt.Printf("\033[41m %v \033[0m\n", err)
// 	// watchdog.UnsetBroken()
// 	// os.Exit(0)
// }
