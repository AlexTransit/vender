package ui

import (
	"fmt"

	"github.com/AlexTransit/vender/hardware/input"
	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/types"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/temoto/alive/v2"
)

func (ui *UI) linesCreate(l1 *string, l2 *string, tuneScreen *bool) {
	c := ui.ms.GetCredit()
	if c == 0 {
		currentLine := ui.g.Hardware.HD44780.Display.GetLine(1)
		*l1 = currentLine
	} else {
		*l1 = ui.g.Config.UI_config.Front.MsgCredit + c.Format100I()
	}
	if len(ui.inputBuf) > 0 {
		*l2 = fmt.Sprintf(ui.g.Config.UI_config.Front.MsgInputCode, string(ui.inputBuf))
		*l1 = ui.g.Config.UI_config.Front.MsgCredit + c.Format100I()
	} else {
		*l2 = " "
	}
	*tuneScreen = false
}

func (ui *UI) parseKeyEvent(e types.Event, l1 *string, l2 *string, tuneScreen *bool, alive *alive.Alive) (nextState types.UiState) {
	sound.PlayKeyBeep()
	rm := tele_api.FromRoboMessage{}
	defer func() {
		if rm.State != 0 {
			go ui.g.Tele.RoboSend(&rm)
		}
	}()
	if input.IsMoneyAbort(&e.Input) {
		alive.Stop()
		ui.g.Log.Infof("money abort event.")
		credit := ui.ms.GetCredit()
		if credit > 0 {
			// FIXME alexm
			sound.PlayFileNoWait("trash.mp3")
			ui.display.SetLines("  :-(", fmt.Sprintf(" -%v", credit.Format100I()))
			err := ui.ms.ReturnMoney()
			ui.g.Error(err)
		}
		return types.StateFrontEnd
	}
	currentState := ui.g.Tele.GetState()
	if input.IsReject(&e.Input) {
		// 	// backspace semantic
		if currentState == tele_api.State_WaitingForExternalPayment {
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
	if currentState == tele_api.State_WaitingForExternalPayment { // ignore key press
		ui.g.Log.Info("qr selected. ignore key")
		*l1 = ui.g.Hardware.HD44780.Display.GetLine(1)
		return types.StateDoesNotChange
	}
	if currentState != tele_api.State_Client {
		rm.State = tele_api.State_Client
	}
	if e.Input.IsTuneKey() {
		*tuneScreen = true
		*l1, *l2 = ui.tuneScreen(e.Input)
		return types.StateDoesNotChange
	}
	if e.Input.IsDigit() || e.Input.IsDot() {
		ui.inputBuf = append(ui.inputBuf, byte(e.Input.Key))
		ui.linesCreate(l1, l2, tuneScreen)
		return types.StateDoesNotChange
	}
	if input.IsAccept(&e.Input) {
		*tuneScreen = false
		if len(ui.inputBuf) == 0 {
			*l1 = ""
			*l2 = ui.g.Config.UI_config.Front.MsgMenuCodeEmpty
			return types.StateDoesNotChange
		}
		// var checkValidCode bool
		// types.UI.FrontResult.Item, checkValidCode = types.UI.Menu[string(ui.inputBuf)]
		mi, checkValidCode := config_global.GetMenuItem(string(ui.inputBuf))
		if !checkValidCode {
			*l1 = ui.g.Config.UI_config.Front.MsgMenuError
			*l2 = ui.g.Config.UI_config.Front.MsgMenuCodeInvalid
			ui.inputBuf = []byte{}
			return types.StateDoesNotChange
		}
		if err := mi.Doer.Validate(); err != nil {
			ui.g.Log.WarningF("validate menu:%v error:%v", mi.Code, err)
			*l1 = ui.g.Config.UI_config.Front.MsgMenuError
			*l2 = ui.g.Config.UI_config.Front.MsgMenuNotAvailable
			ui.inputBuf = []byte{}
			return types.StateDoesNotChange
		}
		credit := ui.ms.GetCredit()
		config_global.VMC.User.SelectedItem = mi
		if mi.Price > credit {
			*l2 = fmt.Sprintf(ui.g.Config.UI_config.Front.MsgInputCode+" "+ui.g.Config.UI_config.Front.MsgPrice, mi.Code, mi.Price.Format100I())
			if credit == 0 {
				*l1 = *ui.sendRequestForQrPayment(&rm)
			} else {
				*l1 = ui.g.Config.UI_config.Front.MsgMenuInsufficientCreditL1
			}
			return types.StateDoesNotChange
		}
		if ui.ms.WaitEscrowAccept(mi.Price) {
			return types.StateDoesNotChange
		}
		return types.StateFrontAccept // success path
	}
	return types.StateDoesNotChange
}

func (ui *UI) parseMoneyEvent(ek types.EventKind) types.UiState {
	sound.PlayMoneyIn()
	config_global.VMC.KeyboardReader(true)
	currentState := ui.g.Tele.GetState()
	if currentState == tele_api.State_WaitingForExternalPayment {
		ui.g.ShowQR("QR disabled. ")
		ui.g.TeleCancelOrder(tele_api.State_Client)
		ui.g.Config.User.PaymenId = 0
	}
	ui.g.Config.User.PaymentMethod = tele_api.PaymentMethod_Cash
	credit := ui.ms.GetCredit()
	price := config_global.VMC.User.SelectedItem.Price
	if price != 0 && credit >= price && config_global.VMC.User.SelectedItem.Doer != nil {
		// menu selected, almost paided and have item doer
		if err := config_global.VMC.User.SelectedItem.Doer.Validate(); err == nil {
			// item valid
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
