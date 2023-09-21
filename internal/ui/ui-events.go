package ui

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/internal/types"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
)

func (ui *UI) linesCreate(l1 *string, l2 *string, tuneScreen *bool) {
	c := ui.ms.GetCredit()
	if c == 0 {
		*l1 = ui.g.Config.UI.Front.MsgStateIntro
	} else {
		*l1 = ui.g.Config.UI.Front.MsgCredit + c.Format100I()
	}
	if len(ui.inputBuf) > 0 {
		*l2 = fmt.Sprintf(ui.g.Config.UI.Front.MsgInputCode, string(ui.inputBuf))
		*l1 = ui.g.Config.UI.Front.MsgCredit + c.Format100I()
	} else {
		*l2 = " "
	}
	*tuneScreen = false
}

func (ui *UI) parseKeyEvent(ctx context.Context, e types.Event, l1 *string, l2 *string, tuneScreen *bool) (nextState types.UiState) {
	if input.IsMoneyAbort(&e.Input) {
		ui.g.Log.Infof("money abort event.")
		credit := ui.ms.GetCredit()
		if credit > 0 {
			ui.display.SetLines("  :-(", fmt.Sprintf(" -%v", credit.Format100I()))
			err := ui.ms.ReturnMoney()
			ui.g.Error(errors.Trace(err))
			ui.cancelQRPay(tele_api.State_Client)
		}
		return types.StateFrontEnd
	}
	if input.IsReject(&e.Input) {
		// 	// backspace semantic
		if types.UI.FrontResult.QRPaymenID != "" {
			ui.cancelQRPay(tele_api.State_Client)
			return types.StateFrontEnd
		}
		if len(ui.inputBuf) >= 1 {
			ui.inputBuf = ui.inputBuf[:len(ui.inputBuf)-1]
		}
		if len(ui.inputBuf) == 0 {
			if ui.ms.GetCredit() == 0 {
				return types.StateFrontEnd
			}
		}
		ui.linesCreate(l1, l2, tuneScreen)
		return types.StateDoesNotChange
	}
	if types.UI.FrontResult.QRPaymenID != "" { // ignore key press
		ui.g.Log.Info("qr selected. ignore key")
		*l1 = types.VMC.HW.Display.L1
		return types.StateDoesNotChange
	}
	go ui.g.ClientBegin(ctx)
	if e.Input.IsTuneKey() {
		*tuneScreen = true
		*l1, *l2 = ui.tuneScreen(e.Input)
		return types.StateDoesNotChange
	}
	if e.Input.IsDigit() || e.Input.IsDot() {
		ui.cancelQRPay(tele_api.State_Client)
		ui.inputBuf = append(ui.inputBuf, byte(e.Input.Key))
		ui.linesCreate(l1, l2, tuneScreen)
		return types.StateDoesNotChange
	}
	if input.IsAccept(&e.Input) {
		*tuneScreen = false
		if types.UI.FrontResult.QRPaymenID != "" {
			return types.StateDoesNotChange
		}
		if len(ui.inputBuf) == 0 {
			*l1 = ui.g.Config.UI.Front.MsgError
			*l2 = ui.g.Config.UI.Front.MsgMenuCodeEmpty
			return types.StateDoesNotChange
		}
		var checkValidCode bool
		types.UI.FrontResult.Item, checkValidCode = types.UI.Menu[string(ui.inputBuf)]
		if !checkValidCode {
			*l1 = ui.g.Config.UI.Front.MsgMenuError
			*l2 = ui.g.Config.UI.Front.MsgMenuCodeInvalid
			ui.inputBuf = []byte{}
			return types.StateDoesNotChange
		}
		if err := types.UI.FrontResult.Item.D.Validate(); err != nil {
			ui.g.Log.Warning("code not valid. code invalid or little ingridient")
			*l1 = ui.g.Config.UI.Front.MsgMenuError
			*l2 = ui.g.Config.UI.Front.MsgMenuNotAvailable
			ui.inputBuf = []byte{}
			return types.StateDoesNotChange
		}
		credit := ui.ms.GetCredit()
		if types.UI.FrontResult.Item.Price > credit {
			*l2 = fmt.Sprintf(ui.g.Config.UI.Front.MsgInputCode+" "+ui.g.Config.UI.Front.MsgPrice, types.UI.FrontResult.Item.Code, types.UI.FrontResult.Item.Price.Format100I())
			if credit == 0 {
				*l1 = *ui.sendRequestForQrPayment()
			} else {
				*l1 = ui.g.Config.UI.Front.MsgMenuInsufficientCredit
			}
			return types.StateDoesNotChange
		}
		if ui.ms.WaitEscrowAccept(types.UI.FrontResult.Item.Price) {
			return types.StateDoesNotChange
		}
		return types.StateFrontAccept // success path
	}
	return types.StateDoesNotChange
}

func (ui *UI) parseMoneyEvent(ek types.EventKind) types.UiState {
	if types.UI.FrontResult.QRPaymenID != "0" {
		ui.cancelQRPay(tele_api.State_Client)
	}
	credit := ui.ms.GetCredit()
	price := types.UI.FrontResult.Item.Price
	if price != 0 && credit >= price && types.UI.FrontResult.Item.D != nil {
		// menu selected, almost paided and have item doer
		if err := types.UI.FrontResult.Item.D.Validate(); err == nil {
			//item valid
			if ek == types.EventMoneyPreCredit {
				// send command escrow to stacker and wait
				ui.ms.BillEscrowToStacker()
				return types.StateDoesNotChange
			}
			return types.StateFrontAccept // success path
		}
	}
	return types.StateDoesNotChange
}
