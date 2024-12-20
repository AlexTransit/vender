package ui

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/mdb/evend"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/types"
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
	sound.PlayFile("started.mp3")
	return types.StateFrontBegin
}

// check current temperature. retunt next state if temperature not correct
func (ui *UI) checkTemperature() (correct bool, stateIfNotCorrect types.UiState) {
	if ui.g.Config.Hardware.Evend.Valve.TemperatureHot != 0 {
		curTemp, e := evend.EValve.GetTemperature()
		if e != nil {
			ui.g.Log.Error(e)
			return false, types.StateBroken
		}
		if curTemp < int32(ui.g.Config.Hardware.Evend.Valve.TemperatureHot-10) {
			line1 := fmt.Sprintf(ui.g.Config.UI.Front.MsgWaterTemp, curTemp)
			evend.Cup.LightOff() // light off
			if types.VMC.HW.Display.L1 != line1 {
				ui.display.SetLines(line1, ui.g.Config.UI.Front.MsgWait)
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
	if types.VMC.NeedRestart { // after upgrade
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
		return types.StateBroken
	}

	var err error
	ui.FrontMaxPrice, err = menuMaxPrice()
	if err != nil {
		ui.g.Error(err)
		return types.StateBroken

	}
	rm := tele_api.FromRoboMessage{State: tele_api.State_Nominal}
	canselQrOrder(&rm)
	types.UI.FrontResult = types.UIMenuResult{
		Cream: DefaultCream,
		Sugar: DefaultSugar,
	}

	if ui.g.Tele.GetState() != tele_api.State_Nominal {
		ui.g.Tele.RoboSend(&rm)
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
		case types.EventInput:
			if nextState := ui.parseKeyEvent(e, &l1, &l2, &tuneScreen); nextState != types.StateDoesNotChange {
				return nextState
			}
		case types.EventMoneyPreCredit, types.EventMoneyCredit:
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
		case types.EventFrontLock: // change state
			return types.StateFrontLock
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
		ui.g.Hardware.Display.Graphic.CopyFile2FB(ui.g.Config.UI.Front.PicQRPayError)
		return &ui.g.Config.UI.Front.MsgNoNetwork
	}
	types.VMC.UiState = uint32(types.StatePrepare)
	rm.State = tele_api.State_WaitingForExternalPayment
	rm.RoboTime = time.Now().Unix()
	rm.Order = &tele_api.Order{
		OrderStatus: tele_api.OrderStatus_waitingForPayment,
		MenuCode:    types.UI.FrontResult.Item.Code,
		Amount:      uint32(types.UI.FrontResult.Item.Price),
	}
	return &ui.g.Config.UI.Front.MsgRemotePayRequest
}

func canselQrOrder(rm *tele_api.FromRoboMessage) {
	if types.UI.FrontResult.PaymenId > 0 {
		rm.Order = &tele_api.Order{
			Amount:      types.UI.FrontResult.QRPayAmount,
			OrderStatus: tele_api.OrderStatus_cancel,
			OwnerInt:    types.UI.FrontResult.PaymenId,
			OwnerType:   tele_api.OwnerType_qrCashLessUser,
		}
		types.UI.FrontResult.PaymenId = 0
	}
}

func (ui *UI) onFrontTune(ctx context.Context) types.UiState {
	// XXX FIXME
	return ui.onFrontSelect(ctx)
}

func (ui *UI) tuneScreen(e types.InputEvent) (l1, l2 string) {
	switch e.Key {
	case input.EvendKeyCreamLess:
		if types.UI.FrontResult.Cream > 0 {
			types.UI.FrontResult.Cream--
		}
	case input.EvendKeyCreamMore:
		if types.UI.FrontResult.Cream < MaxCream {
			types.UI.FrontResult.Cream++
		}
	case input.EvendKeySugarLess:
		if types.UI.FrontResult.Sugar > 0 {
			types.UI.FrontResult.Sugar--
		}
	case input.EvendKeySugarMore:
		if types.UI.FrontResult.Sugar < MaxSugar {
			types.UI.FrontResult.Sugar++
		}
	default:
	}
	var l2b [13]byte
	switch e.Key {
	case input.EvendKeyCreamLess, input.EvendKeyCreamMore:
		l1 = fmt.Sprintf("%s  /%d", ui.g.Config.UI.Front.MsgCream, types.UI.FrontResult.Cream)
		l2b = createScale(types.UI.FrontResult.Cream, MaxCream, DefaultCream)
	case input.EvendKeySugarLess, input.EvendKeySugarMore:
		l1 = fmt.Sprintf("%s  /%d", ui.g.Config.UI.Front.MsgSugar, types.UI.FrontResult.Sugar)
		l2b = createScale(types.UI.FrontResult.Sugar, MaxSugar, DefaultSugar)
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
	ui.g.Tele.RoboSendState(tele_api.State_Process)
	moneysys := money.GetGlobal(ctx)

	selected := types.UI.FrontResult.Item.String()

	ui.g.Log.Debugf("ui-front selected=%s begin", selected)
	if err := moneysys.WithdrawPrepare(ctx, types.UI.FrontResult.Item.Price); err != nil {
		ui.g.Log.Errorf("ui-front CRITICAL error while return change")
	}
	watchdog.DevicesInitializationRequired()
	err := Cook(ctx)
	rm := tele_api.FromRoboMessage{
		Order: &tele_api.Order{
			Amount:        uint32(types.UI.FrontResult.Item.Price),
			PaymentMethod: tele_api.PaymentMethod_Cash,
		},
	}
	OrderMenuAndTune(rm.Order)
	defer ui.g.Tele.RoboSend(&rm)

	if err == nil { // success path
		rm.Order.OrderStatus = tele_api.OrderStatus_complete
		rm.State = tele_api.State_Nominal
		watchdog.SetDeviceInited()
		return types.StateFrontEnd
	}
	moneysys.ReturnDirty()
	rm.State = tele_api.State_Broken
	rm.Err = &tele_api.Err{
		Message: errors.Annotatef(err, "execute %s", selected).Error(),
	}
	if errs := ui.g.Engine.ExecList(ctx, "on_menu_error", ui.g.Config.Engine.OnMenuError); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "on_menu_error"))
	}

	return types.StateBroken
}

func OrderMenuAndTune(o *tele_api.Order) {
	o.MenuCode = types.UI.FrontResult.Item.Code
	o.Cream = types.TuneValueToByte(types.UI.FrontResult.Cream, DefaultCream)
	o.Sugar = types.TuneValueToByte(types.UI.FrontResult.Sugar, DefaultSugar)
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
	// types.VMC.Lock = true
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
	case types.EventFrontLock:
		if types.VMC.UiState == uint32(types.StateBroken) {
			return types.StateBroken
		}
		types.VMC.Lock = false
		return types.StateFrontEnd
	}
	return types.StateFrontEnd
}

// tightly coupled to len(alphabet)=4
func formatScale(value, min, max uint8, alphabet []byte) []byte {
	var vicon [6]byte
	switch value {
	case min:
		vicon[0], vicon[1], vicon[2], vicon[3], vicon[4], vicon[5] = 0, 0, 0, 0, 0, 0
	case max:
		vicon[0], vicon[1], vicon[2], vicon[3], vicon[4], vicon[5] = 3, 3, 3, 3, 3, 3
	default:
		rng := uint16(max) - uint16(min)
		part := uint8((float32(value-min) / float32(rng)) * 24)
		// log.Printf("scale(%d,%d..%d) part=%d", value, min, max, part)
		for i := 0; i < len(vicon); i++ {
			if part >= 4 {
				vicon[i] = 3
				part -= 4
			} else {
				vicon[i] = part
				break
			}
		}
	}
	for i := 0; i < len(vicon); i++ {
		vicon[i] = alphabet[vicon[i]]
	}
	return vicon[:]
}

func ScaleTuneRate(value *uint8, max uint8, center uint8) float32 {
	if *value > max {
		*value = max
	}
	switch {
	case *value == center: // most common path
		return 1
	case *value == 0:
		return 0
	}
	if *value > 0 && *value < center {
		return 1 - (0.25 * float32(center-*value))
	}
	if *value > center && *value <= max {
		return 1 + (0.25 * float32(*value-center))
	}
	panic("code error")
}
