package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/mdb/evend"
	"github.com/AlexTransit/vender/hardware/text_display"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

// type UIMenuResult struct {
// 	Item  MenuItem
// 	Cream uint8
// 	Sugar uint8
// }

func (ui *UI) onFrontBegin(ctx context.Context) State {
	ms := money.GetGlobal(ctx)
	credit := ms.Credit(ctx) / 100
	types.UI.FrontResult = types.UIMenuResult{
		Cream: DefaultCream,
		Sugar: DefaultSugar,
	}

	if credit != 0 {
		ui.g.Error(errors.Errorf("money timeout lost (%v)", credit))
	}
	ms.ResetMoney()
	// XXX FIXME custom business logic creeped into code TODO move to config
	if doCheckTempHot := ui.g.Engine.Resolve("evend.valve.check_temp_hot"); doCheckTempHot != nil && !engine.IsNotResolved(doCheckTempHot) {
		err := doCheckTempHot.Validate()
		if errtemp, ok := err.(*evend.ErrWaterTemperature); ok {
			line1 := fmt.Sprintf(ui.g.Config.UI.Front.MsgWaterTemp, errtemp.Current)
			ui.g.ShowPicture(state.PictureBroken)
			if types.VMC.Client.Light {
				_ = ui.g.Engine.ExecList(ctx, "water-temp", []string{"evend.cup.light_off"})
			}
			if types.VMC.HW.Display.L1 != line1 {
				ui.display.SetLines(line1, ui.g.Config.UI.Front.MsgWait)
				rm := tele_api.FromRoboMessage{
					State:    tele_api.State_TemperatureProblem,
					RoboTime: 0,
					RoboHardware: &tele_api.RoboHardware{
						Temperature: int32(errtemp.Current),
					},
				}

				ui.g.Tele.RoboSend(&rm)
			}
			if e := ui.wait(5 * time.Second); e.Kind == types.EventService {
				return StateServiceBegin
			}
			return StateFrontEnd
		} else if err != nil {
			ui.g.Error(err)
			return StateBroken
		}
	}

	if errs := ui.g.Engine.ExecList(ctx, "on_front_begin", ui.g.Config.Engine.OnFrontBegin); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "on_front_begin"))
		return StateBroken
	}
	ui.g.ClientEnd()

	var err error
	ui.FrontMaxPrice, err = menuMaxPrice()
	if err != nil {
		ui.g.Error(err)
		return StateBroken

	}
	ui.g.Tele.RoboSendState(tele_api.State_Nominal)

	return StateFrontSelect
}

func (ui *UI) onFrontSelect(ctx context.Context) State {
	moneysys := money.GetGlobal(ctx)
	alive := alive.NewAlive()
	defer func() {
		alive.Stop() // stop pending AcceptCredit
		alive.Wait()
	}()
	go moneysys.AcceptCredit(ctx, ui.FrontMaxPrice, alive.StopChan(), ui.eventch)

	for {
	refresh:
		// step 1: refresh display
		credit := moneysys.Credit(ctx)
		if ui.State() == StateFrontTune { // XXX onFrontTune
			goto wait
		}
		ui.frontSelectShow(ctx, credit)

		// step 2: wait for input/timeout
	wait:
		timeout := ui.frontResetTimeout
		if ui.State() == StateFrontTune {
			timeout = modTuneTimeout
		}
		e := ui.wait(timeout)
		switch e.Kind {
		case types.EventInput:
			if input.IsMoneyAbort(&e.Input) {
				ui.g.Log.Infof("money abort event.")
				credit := moneysys.Credit(ctx) / 100
				if credit > 0 {
					ui.display.SetLines("  :-(", fmt.Sprintf(" -%v", credit))
					ui.g.Error(errors.Trace(moneysys.Abort(ctx)))
					ui.cancelQRPay(tele_api.State_Client)
				}
				return StateFrontEnd
			}
			if input.IsReject(&e.Input) {
				// backspace semantic
				if len(ui.inputBuf) > 1 {
					ui.inputBuf = ui.inputBuf[:len(ui.inputBuf)-1]
					goto refresh
				}
				if moneysys.Credit(ctx) != 0 {
					goto refresh
				}
				return StateFrontEnd
			}

			ui.g.ClientBegin()
			switch e.Input.Key {
			case input.EvendKeyCreamLess, input.EvendKeyCreamMore, input.EvendKeySugarLess, input.EvendKeySugarMore:
				// could skip state machine transition and just State=StateFrontTune; goto refresh
				return ui.onFrontTuneInput(e.Input)

			}
			switch {
			case e.Input.IsDigit(), e.Input.IsDot():
				ui.cancelQRPay(tele_api.State_Client)
				ui.inputBuf = append(ui.inputBuf, byte(e.Input.Key))
				goto refresh

			case input.IsAccept(&e.Input):
				if types.UI.FrontResult.QRPaymenID != "" {
					goto wait
				}
				if len(ui.inputBuf) == 0 {
					ui.display.SetLines(ui.g.Config.UI.Front.MsgError, ui.g.Config.UI.Front.MsgMenuCodeEmpty)
					goto wait
				}
				var checkVal bool
				types.UI.FrontResult.Item, checkVal = types.UI.Menu[string(ui.inputBuf)]
				if !checkVal {
					ui.display.SetLines(ui.g.Config.UI.Front.MsgError, ui.g.Config.UI.Front.MsgMenuCodeInvalid)
					goto wait
				}
				credit := moneysys.Credit(ctx)
				mitemString := types.UI.FrontResult.Item.String()
				ui.g.Log.Debugf("mitem=%s validate", mitemString)
				if err := types.UI.FrontResult.Item.D.Validate(); err != nil {
					// ui.g.Log.Errorf("ui-front selected=%s Validate err=%v", mitemString, err)
					ui.display.SetLines(ui.g.Config.UI.Front.MsgError, ui.g.Config.UI.Front.MsgMenuNotAvailable)
					goto wait
				}
				ui.g.Log.Debugf("compare price=%v credit=%v", types.UI.FrontResult.Item.Price, credit)
				if types.UI.FrontResult.Item.Price > credit {
					var l1, l2 string
					if credit == 0 {
						types.UI.FrontResult.QRPaymenID = "wait"
						types.VMC.EvendKeyboardInput(false)
						l2 = fmt.Sprintf(ui.g.Config.UI.Front.MsgRemotePayL2, types.UI.FrontResult.Item.Code, types.UI.FrontResult.Item.Price.Format100I())
						ui.sendRequestForQrPayment()
					} else {
						l1 = ui.g.Config.UI.Front.MsgMenuInsufficientCreditL1
						l2 = fmt.Sprintf(ui.g.Config.UI.Front.MsgMenuInsufficientCreditL2, credit.Format100I(), types.UI.FrontResult.Item.Price.Format100I())
					}
					ui.display.SetLines(l1, l2)
					goto wait
				}

				return StateFrontAccept // success path

			default:
				ui.g.Log.Errorf("ui-front unhandled input=%v", e)
				return StateFrontSelect
			}

		case types.EventMoneyCredit:
			credit := moneysys.Credit(ctx)
			if types.UI.FrontResult.QRPaymenID == "wait" {
				ui.cancelQRPay(tele_api.State_Client)
				// types.VMC.EvendKeyboardInput(true)
			}
			price := types.UI.FrontResult.Item.Price 
			if price != 0 && credit >= price {
				if types.UI.FrontResult.Item.D == nil {
					goto wait
				}
				if err := types.UI.FrontResult.Item.D.Validate(); err == nil {
					return StateFrontAccept // success path
				}
			}

			go moneysys.AcceptCredit(ctx, ui.FrontMaxPrice, alive.StopChan(), ui.eventch)

		case types.EventService:
			return StateServiceBegin

		case types.EventTime:
			if ui.State() == StateFrontTune { // XXX onFrontTune
				return StateFrontSelect // "return to previous mode"
			}
			return StateFrontTimeout

		case types.EventFrontLock:
			return StateFrontLock

		case types.EventBroken:
			return StateBroken

		case types.EventLock, types.EventStop:
			return StateFrontEnd

		default:
			panic(fmt.Sprintf("code error state=%v unhandled event=%v", ui.State(), e))
		}
	}
}

func (ui *UI) sendRequestForQrPayment() {
	types.VMC.UiState = uint32(StatePrepare)
	rm := tele_api.FromRoboMessage{
		State:    tele_api.State_WaitingForExternalPayment,
		RoboTime: time.Now().Unix(),
		Order: &tele_api.Order{
			OrderStatus: tele_api.OrderStatus_waitingForPayment,
			MenuCode:    types.UI.FrontResult.Item.Code,
			Amount:      uint32(types.UI.FrontResult.Item.Price),
		},
	}
	ui.g.Tele.RoboSend(&rm)
}
func (ui *UI) cancelQRPay(s tele_api.State) {
	defer func() {
		types.UI.FrontResult.QRPaymenID = ""
	}()
	if types.UI.FrontResult.QRPaymenID == "" || types.UI.FrontResult.QRPaymenID == "wait" {
		return
	}
	rm := tele_api.FromRoboMessage{
		State: s,
		Order: &tele_api.Order{
			Amount:      types.UI.FrontResult.QRPayAmount,
			OrderStatus: tele_api.OrderStatus_cancel,
			OwnerStr:    types.UI.FrontResult.QRPaymenID,
			OwnerType:   tele_api.OwnerType_qrCashLessUser,
		},
	}
	ui.g.Tele.RoboSend(&rm)
}

func (ui *UI) frontSelectShow(ctx context.Context, credit currency.Amount) {
	config := ui.g.Config.UI.Front
	l1 := config.MsgStateIntro
	l2 := ""
	if (credit != 0) || (len(ui.inputBuf) > 0) {
		l1 = ui.g.Config.UI.Front.MsgCredit + credit.FormatCtx(ctx)
		l2 = fmt.Sprintf(ui.g.Config.UI.Front.MsgInputCode, string(ui.inputBuf))
	}
	ui.display.SetLines(l1, l2)
}

func (ui *UI) onFrontTune(ctx context.Context) State {
	// XXX FIXME
	return ui.onFrontSelect(ctx)
}

func (ui *UI) onFrontTuneInput(e types.InputEvent) State {
	switch e.Key {
	case input.EvendKeyCreamLess:
		if types.UI.FrontResult.Cream > 0 {
			types.UI.FrontResult.Cream--
			//lint:ignore SA9003 empty branch
		} else {
			// TODO notify "impossible input" (sound?)
		}
	case input.EvendKeyCreamMore:
		if types.UI.FrontResult.Cream < MaxCream {
			types.UI.FrontResult.Cream++
			//lint:ignore SA9003 empty branch
		} else {
			// TODO notify "impossible input" (sound?)
		}
	case input.EvendKeySugarLess:
		if types.UI.FrontResult.Sugar > 0 {
			types.UI.FrontResult.Sugar--
			//lint:ignore SA9003 empty branch
		} else {
			// TODO notify "impossible input" (sound?)
		}
	case input.EvendKeySugarMore:
		if types.UI.FrontResult.Sugar < MaxSugar {
			types.UI.FrontResult.Sugar++
			//lint:ignore SA9003 empty branch
		} else {
			// TODO notify "impossible input" (sound?)
		}
	default:
		return StateFrontSelect
	}
	var t1, t2 []byte
	next := StateFrontSelect
	switch e.Key {
	case input.EvendKeyCreamLess, input.EvendKeyCreamMore:
		t1 = ui.display.Translate(fmt.Sprintf("%s  /%d", ui.g.Config.UI.Front.MsgCream, types.UI.FrontResult.Cream))
		t2 = formatScale(types.UI.FrontResult.Cream, 0, MaxCream, ScaleAlpha)
		next = StateFrontTune
	case input.EvendKeySugarLess, input.EvendKeySugarMore:
		t1 = ui.display.Translate(fmt.Sprintf("%s  /%d", ui.g.Config.UI.Front.MsgSugar, types.UI.FrontResult.Sugar))
		t2 = formatScale(types.UI.FrontResult.Sugar, 0, MaxSugar, ScaleAlpha)
		next = StateFrontTune
	default:
		fmt.Printf("\n\033[41m как он может сработать2? \033[0m\n\n")
		return StateFrontSelect
	}
	t2 = append(append(append(make([]byte, 0, text_display.MaxWidth), '-', ' '), t2...), ' ', '+')
	ui.display.SetLinesBytes(
		ui.display.JustCenter(t1),
		ui.display.JustCenter(t2),
	)
	return next
}

func (ui *UI) onFrontAccept(ctx context.Context) State {
	ui.g.Tele.RoboSendState(tele_api.State_Process)
	// ui.g.Hardware.Input.Enable(false)
	moneysys := money.GetGlobal(ctx)
	uiConfig := &ui.g.Config.UI

	selected := types.UI.FrontResult.Item.String()
	rm := CreateOrderMessageAndFillSelected()
	if moneysys.GetGiftCredit() == 0 {
		// teletx.PaymentMethod = tele_api.PaymentMethod_Cash
		rm.Order.PaymentMethod = tele_api.PaymentMethod_Cash
	} else {
		// teletx.PaymentMethod = tele_api.PaymentMethod_Gift
		rm.Order.PaymentMethod = tele_api.PaymentMethod_Gift
	}

	ui.g.Log.Debugf("ui-front selected=%s begin", selected)
	if err := moneysys.WithdrawPrepare(ctx, types.UI.FrontResult.Item.Price); err != nil {
		ui.g.Log.Errorf("ui-front CRITICAL error while return change")
	}
	err := Cook(ctx)
	defer ui.g.Tele.RoboSend(&rm)
	// rm.Stock.Stocks

	if err == nil { // success path
		rm.Order.OrderStatus = tele_api.OrderStatus_complete
		rm.State = tele_api.State_Nominal
		return StateFrontEnd
	}
	rm.State = tele_api.State_Broken
	ui.display.SetLines(uiConfig.Front.MsgError, uiConfig.Front.MsgMenuError)
	rm.Err = &tele_api.Err{
		Message: errors.Annotatef(err, "execute %s", selected).Error(),
	}
	if errs := ui.g.Engine.ExecList(ctx, "on_menu_error", ui.g.Config.Engine.OnMenuError); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "on_menu_error"))
	}

	return StateBroken
}

func CreateOrderMessageAndFillSelected() tele_api.FromRoboMessage {
	rm := tele_api.FromRoboMessage{
		Order: &tele_api.Order{
			MenuCode: types.UI.FrontResult.Item.Code,
			Cream:    types.TuneValueToByte(types.UI.FrontResult.Cream, DefaultCream),
			Sugar:    types.TuneValueToByte(types.UI.FrontResult.Sugar, DefaultSugar),
			Amount:   uint32(types.UI.FrontResult.Item.Price),
		},
	}
	return rm
}

func (ui *UI) onFrontTimeout(ctx context.Context) State {
	// ui.g.Log.Debugf("ui state=%s result=%#v", ui.State().String(), ui.FrontResult)
	// moneysys := money.GetGlobal(ctx)
	// moneysys.save
	return StateFrontEnd
}

func (ui *UI) onFrontLock() State {
	// ui.g.Hardware.Input.Enable(false)
	// types.VMC.Lock = true
	// ui.display.SetLines(ui.g.Config.UI.Front.MsgStateLocked, "")
	timeout := ui.frontResetTimeout
	e := ui.wait(timeout)
	switch e.Kind {
	case types.EventService:
		return StateServiceBegin
	case types.EventTime:
		if ui.State() == StateFrontTune { // XXX onFrontTune
			return StateFrontSelect // "return to previous mode"
		}
		return StateFrontTimeout
	case types.EventBroken:
		return StateBroken
	case types.EventFrontLock:
		if types.VMC.UiState == 2 { // broken. fix this
			return StateBroken
		}
		types.VMC.Lock = false
		return StateFrontEnd
	}
	return StateFrontEnd
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

func ScaleTuneRate(value, max, center uint8) float32 {
	switch {
	case value == center: // most common path
		return 1
	case value == 0:
		return 0
	}
	if value > max {
		value = max
	}
	if value > 0 && value < center {
		return 1 - (0.25 * float32(center-value))
	}
	if value > center && value <= max {
		return 1 + (0.25 * float32(value-center))
	}
	panic("code error")
}

func menuMaxPrice() (currency.Amount, error) {
	max := currency.Amount(0)
	empty := true
	for _, item := range types.UI.Menu {
		valErr := item.D.Validate()
		if valErr == nil {
			empty = false
			if item.Price > max {
				max = item.Price
			}
		}
	}
	if empty {
		return 0, errors.Errorf("menu len=%d no valid items", len(types.UI.Menu))
	}
	return max, nil
}
