package ui

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/mdb/evend"
	"github.com/AlexTransit/vender/helpers"
	config_global "github.com/AlexTransit/vender/internal/config"
	menu_vmc "github.com/AlexTransit/vender/internal/menu"
	"github.com/AlexTransit/vender/internal/menu/menu_config"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/types"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	"github.com/AlexTransit/vender/internal/watchdog"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
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
			line1 := fmt.Sprintf(ui.g.Config.UI_config.Front.MsgWaterTemp, curTemp)
			evend.Cup.LightOff() // light off
			if config_global.VMC.User.TDL1 != line1 {
				ui.display.SetLines(line1, ui.g.Config.UI_config.Front.MsgWait)
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
	if config_global.VMC.Engine.NeedRestart { // after upgrade
		ui.g.VmcStopWOInitRequared(ctx)
		return types.StateStop
	}
	if valid, nextState := ui.checkTemperature(); !valid {
		return nextState
	}
	watchdog.Refresh()
	credit := ui.ms.GetCredit() / 100
	if credit != 0 {
		ui.g.Error(errors.Errorf("money timeout lost (%v)", credit))
	}
	ui.ms.ResetMoney()

	ui.g.ClientEnd(ctx)
	runtime.GC() // чистка мусора в памяти
	if errs := ui.g.Engine.ExecList(ctx, "on_front_begin", ui.g.Config.Engine.OnFrontBegin); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "on_front_begin"))
		watchdog.SetBroken()
		return types.StateBroken
	}

	var err error
	ui.FrontMaxPrice, err = menu_vmc.MenuMaxPrice()
	if err != nil {
		ui.g.Error(err)
		watchdog.SetBroken()
		return types.StateBroken

	}
	ui.g.Tele.RoboSendState(tele_api.State_Nominal)
	config_global.VMC.User = ui_config.UIUser{
		KeyboardReadEnable: true,
		UIMenuStruct: menu_config.UIMenuStruct{
			Cream: config_global.VMC.Engine.Menu.DefaultCream,
			Sugar: config_global.VMC.Engine.Menu.DefaultSugar,
		},
	}
	return types.StateFrontSelect
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
			if nextState := ui.parseKeyEvent(e, &l1, &l2, &tuneScreen); nextState != types.StateDoesNotChange {
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
		case types.EventLock, types.EventStop: // change state
			return types.StateFrontEnd
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

func canselQrOrder(rm *tele_api.FromRoboMessage) {
	if config_global.VMC.User.PaymenId > 0 {
		rm.Order = &tele_api.Order{
			Amount:      config_global.VMC.User.QRPayAmount,
			OrderStatus: tele_api.OrderStatus_cancel,
			OwnerInt:    config_global.VMC.User.PaymenId,
			OwnerType:   tele_api.OwnerType_qrCashLessUser,
		}
		config_global.VMC.User.PaymenId = 0
	}
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
	// ui.g.Tele.RoboSendState(tele_api.State_Process)
	ui.g.SendCooking()
	moneysys := money.GetGlobal(ctx)

	selected := config_global.VMC.User.SelectedItem.String()

	// FIXME AlexM заглушка пока не переделал
	if config_global.VMC.User.PaymentMethod == 0 {
		ui.g.Log.Debugf("ui-front selected=%s begin", selected)
		if err := moneysys.WithdrawPrepare(ctx, config_global.VMC.User.SelectedItem.Price); err != nil {
			ui.g.Log.Errorf("ui-front CRITICAL error while return change")
		}
		config_global.VMC.User.PaymentMethod = tele_api.PaymentMethod_Cash
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
		return types.StateFrontEnd
	}

	// ошибка при приготовлении
	if config_global.VMC.User.PaymentMethod == tele_api.PaymentMethod_Cash {
		moneysys.ReturnDirty()
	}
	ui.g.SendBroken("execute " + selected + err.Error())
	watchdog.SetBroken()
	return types.StateBroken
}

// func OrderMenuAndTune(o *tele_api.Order) {
// 	o.MenuCode = config_global.VMC.User.SelectedItem.Code
// 	o.Cream = TuneValueToByte(config_global.VMC.User.Cream, config_global.VMC.Engine.Menu.DefaultCream)
// 	o.Sugar = TuneValueToByte(config_global.VMC.User.Sugar, config_global.VMC.Engine.Menu.DefaultCream)
// }

func TuneValueToByte(currentValue uint8, defaultValue uint8) []byte {
	if currentValue == defaultValue {
		return nil
	}
	return []byte{currentValue + 1}
}

func (ui *UI) onFrontTimeout(ctx context.Context) types.UiState {
	// ui.g.Log.Debugf("ui state=%s result=%#v", ui.State().String(), ui.FrontResult)
	// moneysys := money.GetGlobal(ctx)
	// moneysys.save
	_ = ctx
	return types.StateFrontEnd
}

func (ui *UI) onFrontLock() types.UiState {
	// ui.g.Hardware.Input.Enable(false)
	// ui.display.SetLines(ui.g.Config.UI.Front.MsgStateLocked, "")
	timeout := ui.frontResetTimeout
	e := ui.wait(timeout)
	switch e.Kind {
	case types.EventService:
		return types.StateServiceBegin
	case types.EventTime:
		// if ui.State() == StateFrontTune { // XXX onFrontTune
		// 	return StateFrontSelect // "return to previous mode"
		// }
		return types.StateFrontTimeout
	case types.EventBroken:
		return types.StateBroken
		// case types.EventFrontLock:
		// 	if config_global.VMC.UIState() == uint32(types.StateBroken) {
		// 		return types.StateBroken
		// 	}
		// 	config_global.VMC.User.Lock = false
		// 	return types.StateFrontEnd
	}
	return types.StateFrontEnd
}

// // tightly coupled to len(alphabet)=4
// func formatScale(value, min, max uint8, alphabet []byte) []byte {
// 	var vicon [6]byte
// 	switch value {
// 	case min:
// 		vicon[0], vicon[1], vicon[2], vicon[3], vicon[4], vicon[5] = 0, 0, 0, 0, 0, 0
// 	case max:
// 		vicon[0], vicon[1], vicon[2], vicon[3], vicon[4], vicon[5] = 3, 3, 3, 3, 3, 3
// 	default:
// 		rng := uint16(max) - uint16(min)
// 		part := uint8((float32(value-min) / float32(rng)) * 24)
// 		// log.Printf("scale(%d,%d..%d) part=%d", value, min, max, part)
// 		for i := 0; i < len(vicon); i++ {
// 			if part >= 4 {
// 				vicon[i] = 3
// 				part -= 4
// 			} else {
// 				vicon[i] = part
// 				break
// 			}
// 		}
// 	}
// 	for i := 0; i < len(vicon); i++ {
// 		vicon[i] = alphabet[vicon[i]]
// 	}
// 	return vicon[:]
// }
