package ui

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/mdb/evend"
	config_global "github.com/AlexTransit/vender/internal/config"
	menu_vmc "github.com/AlexTransit/vender/internal/menu"
	"github.com/AlexTransit/vender/internal/menu/menu_config"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/types"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	"github.com/AlexTransit/vender/internal/watchdog"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/temoto/alive/v2"
)

//	type UIMenuResult struct {
//		Item  MenuItem
//		Cream uint8
//		Sugar uint8
//	}
func (ui *UI) onFrontStart() types.UiState {
	watchdog.Refresh()
	if ok, nextState := ui.checkTemperature(); !ok {
		return nextState
	}
	// FIXME alexm
	sound.PlayFileNoWait("started.mp3")
	ui.g.Tele.RoboSendState(tele_api.State_Nominal)
	return types.StateFrontBegin
}

// check current temperature. retunt next state if temperature not correct
func (ui *UI) checkTemperature() (correct bool, stateIfNotCorrect types.UiState) {
	/* test
	if true {
		return true, 0
	}
	//*/
	if ui.g.Config.Hardware.Evend.Valve.TemperatureHot != 0 {
		curTemp, e := evend.EValve.GetTemperature()
		if e != nil {
			ui.g.Log.Error(e)
			return false, types.StateBroken
		}
		if curTemp < int32(ui.g.Config.Hardware.Evend.Valve.TemperatureHot-10) {
			line2 := fmt.Sprintf(ui.g.Config.UI_config.Front.MsgWaterTemp, curTemp)
			evend.Cup.LightOff() // light off
			if ui.g.Hardware.HD44780.Display.GetLine(2) != line2 {
				ui.display.SetLines(ui.g.Config.UI_config.Front.MsgWait, line2)
				rm := tele_api.FromRoboMessage{
					State: tele_api.State_TemperatureProblem,
					RoboHardware: &tele_api.RoboHardware{
						Temperature: curTemp,
					},
				}
				ui.g.Tele.RoboSend(&rm)
			}
			if e := ui.wait(5 * time.Second); e.Kind == types.EventService {
				return false, types.StateServiceBegin
			}
			return false, types.StateOnStart
		}
	}

	return true, 0
}

func (ui *UI) onFrontBegin(ctx context.Context) types.UiState {
	ui.g.TeleCancelOrder(tele_api.State_Nominal) // if order not complete, send cancel order and nominal state
	ui.RefreshUserPresets()
	if config_global.VMC.Engine.NeedRestart { // after upgrade
		ui.g.GlobalError = "triger restart"
		ui.g.VmcStopWOInitRequared(ctx)
		return types.StateStop
	}
	if valid, nextState := ui.checkTemperature(); !valid {
		return nextState
	}
	watchdog.Refresh()
	credit := ui.ms.GetCredit() / 100
	if credit != 0 {
		ui.g.Log.Errorf("money timeout lost (%v)", credit)
	}
	ui.ms.ResetMoney()

	ui.g.ClientEnd(ctx)
	runtime.GC() // чистка мусора в памяти
	if errs := ui.g.Engine.ExecList(ctx, "on_front_begin", ui.g.Config.Engine.OnFrontBegin); len(errs) != 0 {
		ui.g.Log.Errorf("on_front_begin (%v)", errs)
		return types.StateBroken
	}

	var err error
	ui.FrontMaxPrice, err = menu_vmc.MenuMaxPrice()
	if err != nil {
		ui.g.Error(err)
		return types.StateBroken

	}
	return types.StateFrontSelect
}

func (ui *UI) RefreshUserPresets() {
	config_global.VMC.User = ui_config.UIUser{
		KeyboardReadEnable: true,
		UIMenuStruct: menu_config.UIMenuStruct{
			Cream: config_global.VMC.Engine.Menu.DefaultCream,
			Sugar: config_global.VMC.Engine.Menu.DefaultSugar,
		},
	}
}

func (ui *UI) onFrontSelect(ctx context.Context) types.UiState {
	alive := alive.NewAlive()
	alive.Add(2)
	go ui.ms.AcceptCredit(ctx, ui.FrontMaxPrice, alive, ui.eventch)
	defer func() {
		alive.Stop() // stop pending AcceptCredit
		alive.Wait()
	}()
	l1 := ui.display.GetLine(1)
	// ui.g.Config.UI.Front.MsgStateIntro
	l2 := ui.display.GetLine(2)
	tuneScreen := false
	for {
		ui.display.SetLines(l1, l2)
		timeout := ui.frontResetTimeout
		if tuneScreen {
			timeout = modTuneTimeout
		}
		e := ui.wait(timeout)
		switch e.Kind {
		case types.EventAccept:
			return types.StateFrontAccept
		case types.EventInput: // from keyboard
			if nextState := ui.parseKeyEvent(e, &l1, &l2, &tuneScreen, alive); nextState != types.StateDoesNotChange {
				return nextState
			}
		case types.EventMoneyPreCredit, types.EventMoneyCredit: // from validators
			if nextState := ui.parseMoneyEvent(e.Kind); nextState != types.StateDoesNotChange {
				return nextState
			}
			ui.linesCreate(&l1, &l2, &tuneScreen)
		case types.EventTime:
			if tuneScreen {
				ui.linesCreate(&l1, &l2, &tuneScreen) // disable tune screem
			} else {
				return types.StateFrontTimeout
			}
		case types.EventService: // change state
			return types.StateServiceBegin
		// case types.EventFrontLock: // change state
		// 	return types.StateFrontLock
		case types.EventBroken: // change state
			return types.StateBroken
		case types.EventLock:
			return types.StateLocked
		case types.EventStop: // change state
			return types.StateStop
		default: // destroy program
			panic(fmt.Sprintf("code error state=%v unhandled event=%v", ui.State(), e))
		}
	}
}

// send request for pay ( if posible ) and
// return message for display
func (ui *UI) sendRequestForQrPayment(rm *tele_api.FromRoboMessage) (message_for_display *string) {
	if !ui.g.Tele.RoboConnected() {
		ui.g.Hardware.Display.Graphic.CopyFile2FB(ui.g.Config.UI_config.Front.PicQRPayError)
		return &ui.g.Config.UI_config.Front.MsgNoNetwork
	}
	config_global.VMC.UIState(uint32(types.StatePrepare))
	rm.State = tele_api.State_WaitingForExternalPayment
	rm.RoboTime = time.Now().Unix()
	rm.Order = &tele_api.Order{
		OrderStatus: tele_api.OrderStatus_waitingForPayment,
		MenuCode:    config_global.VMC.User.SelectedItem.Code,
		Amount:      uint32(config_global.VMC.User.SelectedItem.Price),
	}
	return &ui.g.Config.UI_config.Front.MsgRemotePayRequest
}

func (ui *UI) onFrontTune(ctx context.Context) types.UiState {
	// XXX FIXME
	return ui.onFrontSelect(ctx)
}

func (ui *UI) tuneScreen(e types.InputEvent) (l1, l2 string) {
	switch e.Key {
	case input.EvendKeyCreamLess:
		if config_global.VMC.User.Cream > 0 {
			config_global.VMC.User.Cream--
		}
	case input.EvendKeyCreamMore:
		if config_global.VMC.User.Cream < config_global.CreamMax() {
			config_global.VMC.User.Cream++
		}
	case input.EvendKeySugarLess:
		if config_global.VMC.User.Sugar > 0 {
			config_global.VMC.User.Sugar--
		}
	case input.EvendKeySugarMore:
		if config_global.VMC.User.Sugar < config_global.SugarMax() {
			config_global.VMC.User.Sugar++
		}
	default:
	}
	var l2b [13]byte
	switch e.Key {
	case input.EvendKeyCreamLess, input.EvendKeyCreamMore:
		l1 = fmt.Sprintf("%s  /%d", ui.g.Config.UI_config.Front.MsgCream, config_global.VMC.User.Cream)
		l2b = createScale(config_global.VMC.User.Cream, config_global.CreamMax(), config_global.DefaultCream())
	case input.EvendKeySugarLess, input.EvendKeySugarMore:
		l1 = fmt.Sprintf("%s  /%d", ui.g.Config.UI_config.Front.MsgSugar, config_global.VMC.User.Sugar)
		l2b = createScale(config_global.VMC.User.Sugar, config_global.SugarMax(), config_global.DefaultSugar())
	default:
	}
	l2 = string(l2b[:])
	return l1, l2
}

func createScale(currentValue uint8, maximumValue uint8, defaultValue uint8) (ba [13]byte) {
	ba = [13]byte{'-', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', '+'}
	for i := uint8(2); i <= maximumValue+2; i++ {
		ba[i] = 0x3d
	}
	ba[defaultValue+2] = []byte(`"`)[0] // default char
	ba[currentValue+2] = []byte("#")[0] // current char
	return ba
}

func (ui *UI) onFrontAccept(ctx context.Context) types.UiState {
	ui.g.SendCooking()
	moneysys := money.GetGlobal(ctx)
	selected := config_global.VMC.User.SelectedItem.Code

	// FIXME AlexM заглушка пока не переделал
	if config_global.VMC.User.PaymentMethod == tele_api.PaymentMethod_Cash {
		ui.g.Log.Debugf("ui-front selected=%s begin", selected)
		if err := moneysys.WithdrawPrepare(ctx, config_global.VMC.User.SelectedItem.Price); err != nil {
			ui.g.Log.Errorf("ui-front CRITICAL error while return change")
		}
	}
	watchdog.DevicesInitializationRequired()
	err := menu_vmc.Cook(ctx)
	if err == nil { // success path
		watchdog.SetDeviceInited()
		rm := tele_api.FromRoboMessage{
			State: tele_api.State_Nominal,
			Order: &tele_api.Order{
				MenuCode:      config_global.VMC.User.SelectedItem.Code,
				Cream:         TuneValueToByte(config_global.VMC.User.Cream, config_global.VMC.Engine.Menu.DefaultCream),
				Sugar:         TuneValueToByte(config_global.VMC.User.Sugar, config_global.VMC.Engine.Menu.DefaultCream),
				Amount:        uint32(config_global.VMC.User.SelectedItem.Price),
				OrderStatus:   tele_api.OrderStatus_complete,
				PaymentMethod: config_global.VMC.User.PaymentMethod,
				OwnerInt:      config_global.VMC.User.PaymenId,
				OwnerType:     config_global.VMC.User.PaymentType,
			},
		}
		ui.g.Tele.RoboSend(&rm)
		ui.RefreshUserPresets()
		return types.StateFrontEnd
	}

	// ошибка при приготовлении
	ui.g.GlobalError = fmt.Sprintf("execute code:%s %v", selected, err)
	if config_global.VMC.User.PaymentMethod == tele_api.PaymentMethod_Cash {
		moneysys.ReturnDirty()
	}
	return types.StateBroken
}

func TuneValueToByte(currentValue uint8, defaultValue uint8) []byte {
	if currentValue == defaultValue {
		return nil
	}
	return []byte{currentValue + 1}
}

func (ui *UI) onFrontTimeout(_ context.Context) types.UiState {
	return types.StateFrontEnd
}

func (ui *UI) onFrontLock() types.UiState {
	timeout := ui.frontResetTimeout
	e := ui.wait(timeout)
	switch e.Kind {
	case types.EventService:
		return types.StateServiceBegin
	case types.EventTime:
		return types.StateFrontTimeout
	case types.EventBroken:
		return types.StateBroken
	}
	return types.StateFrontEnd
}
