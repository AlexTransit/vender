package tele

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/ui"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/golang/protobuf/proto"
	"github.com/juju/errors"
)

var (
	errInvalidArg = fmt.Errorf("invalid arg")
)

// AlexM old tele
func (t *tele) onCommandMessage(ctx context.Context, payload []byte) bool {
	if t.currentState == tele_api.State_Invalid || t.currentState == tele_api.State_Boot {
		return true
	}
	cmd := new(tele_api.Command)
	err := proto.Unmarshal(payload, cmd)
	if err != nil {
		t.log.Errorf("tele command parse raw=%x err=%v", payload, err)
		// TODO reply error
		return true
	}
	t.log.Debugf("tele command raw=%x task=%#v", payload, cmd.String())

	if err = t.dispatchCommand(ctx, cmd); err != nil {
		t.log.Errorf("command message error (%v)", err)
		return true
	}

	return true
}

func (t *tele) messageForRobot(ctx context.Context, payload []byte) bool {
	if t.currentState == 0 {
		return false
	}
	err := proto.Unmarshal(payload, &t.InMessage)

	if err != nil {
		t.log.Errorf("tele command parse raw=%x err=%v", payload, err)
		return false
	}
	t.log.Infof("incoming message:%v", t.InMessage)
	switch t.InMessage.Cmd {
	case tele_api.MessageType_makeOrder:
		t.mesageMakeOrger(ctx)
	case tele_api.MessageType_reportState:
		t.RoboSend(&tele_api.FromRoboMessage{
			State: t.currentState,
		})
	case tele_api.MessageType_reportStock:
	case tele_api.MessageType_showQR:
		t.messageShowQr(ctx)
	case tele_api.MessageType_executeCommand:
	default: // unknow mesage type
	}
	return true
}
func (t *tele) mesageMakeOrger(ctx context.Context) {
	if t.InMessage.MakeOrder == nil || types.UI.FrontResult.PaymenId == t.InMessage.MakeOrder.OwnerInt {
		t.log.Err("ignore mesageMakeOrger (%v)", t.InMessage.MakeOrder)
		return
	}
	// prepare output message
	g := state.GetGlobal(ctx)
	t.OutMessage = tele_api.FromRoboMessage{
		Order: &tele_api.Order{},
	}
	t.OutMessage.Order.OwnerInt = t.InMessage.MakeOrder.OwnerInt
	t.OutMessage.Order.OwnerStr = t.InMessage.MakeOrder.OwnerStr
	t.OutMessage.Order.OwnerType = t.InMessage.MakeOrder.OwnerType
	t.OutMessage.Order.PaymentMethod = t.InMessage.MakeOrder.PaymentMethod
	defer t.RoboSend(&t.OutMessage)
	rt := time.Now().Unix()
	st := t.InMessage.ServerTime
	if rt-st > 180 {
		// затычка по тайм ауту. если команда пришла с задержкой
		errM := fmt.Sprintf("remote make error. big time difference between server and robot. RTime:%v STime:%v", time.Unix(rt, 0), time.Unix(st, 0))
		t.OutMessage.Err = &tele_api.Err{Message: errM}
		t.log.Errorf(errM)
		t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_orderError
		types.VMC.EvendKeyboardInput(true)
		l1 := g.Config.UI.Front.MsgMenuInsufficientCredit
		l2 := fmt.Sprintf(g.Config.UI.Front.MsgInputCode, types.UI.FrontResult.Item.Code)
		// l2 := fmt.Sprintf(g.Config.UI.Front.MsgMenuInsufficientCreditL2, "0", types.UI.FrontResult.Item.Price.Format100I())
		g.Hardware.HD44780.Display.SetLines(l1, l2)
		g.ShowPicture(state.PictureClient)
		return
	}

	switch t.InMessage.MakeOrder.OrderStatus {
	case tele_api.OrderStatus_doSelected:
		// make selected code. payment via QR, etc
		t.log.Infof("message make doSelected. robo state:%v", t.currentState)
		if t.currentState != tele_api.State_WaitingForExternalPayment {
			// if t.currentState != tele_api.State_WaitingForExternalPayment || types.UI.FrontResult.QRPaymenID != t.InMessage.MakeOrder.OwnerStr {
			t.log.Errorf("doSelected t.currentState != tele_api.State_WaitingForExternalPayment (%v != %v) or types.UI.FrontResult.QRPaymenID != t.InMessage.MakeOrder.OwnerStr (%v !=%v)", t.currentState, tele_api.State_WaitingForExternalPayment, types.UI.FrontResult.QRPaymenID, t.InMessage.MakeOrder.OwnerStr)
			t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_orderError
			return
		}
		t.OutMessage.Order.Amount = t.InMessage.MakeOrder.Amount
		types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price

	case tele_api.OrderStatus_doTransferred:
		//TODO execute external order. сделать внешний заказ
		// check validity, price. проверить валидность, цену
		if t.currentState != tele_api.State_Nominal {
			t.log.Errorf("make doTransferred unposible. robo state:%v", t.currentState)
			t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_robotIsBusy
			return
		}
		// when paying by balance, the current balance of the client is sent. при оплате по балансу, присылвется текущий баланс клиента
		if t.InMessage.MakeOrder.PaymentMethod != tele_api.PaymentMethod_Balance {
			t.log.Errorf("make doTransferred unposible. PaymentMethod not balanse(%v)", t.currentState)
			t.OutMessage.Err.Message = "command remote cook, not set balance"
			return
		}
		if !t.checkCodePriceValid(&t.InMessage.MakeOrder.MenuCode, t.InMessage.MakeOrder.Amount) {
			t.log.Errorf("checkCodePriceValid not valid(%v)", t.InMessage.MakeOrder)
			return
		}
		types.UI.FrontResult.Sugar = tuneCook(t.InMessage.MakeOrder.Sugar, ui.DefaultSugar, ui.SugarMax())
		types.UI.FrontResult.Cream = tuneCook(t.InMessage.MakeOrder.Cream, ui.DefaultCream, ui.CreamMax())
		t.OutMessage.Order.Amount = uint32(types.UI.FrontResult.Item.Price)
		types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price

	default: //unknown status
		t.log.Errorf("unknown order status(%v)", t.InMessage.MakeOrder.OrderStatus)
		t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_orderError
		return
	}
	om := t.OutMessage

	go func() { // run cooking
		t.RemCook(ctx, om)
		g.LockCh <- struct{}{}
	}()
	// defer send OrderStatus_executionStart
	t.OutMessage.State = tele_api.State_RemoteControl
	t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_executionStart
}

func (t *tele) messageShowQr(ctx context.Context) {
	if t.InMessage.ShowQR == nil {
		return
	}
	g := state.GetGlobal(ctx)
	switch t.InMessage.ShowQR.QrType {
	case tele_api.ShowQR_order:
		if types.UI.FrontResult.QRPaymenID == "0" {
			types.UI.FrontResult.QRPaymenID = t.InMessage.ShowQR.DataStr
			types.UI.FrontResult.QRPayAmount = uint32(t.InMessage.ShowQR.DataInt)
			g.ShowQR(t.InMessage.ShowQR.QrText)
			l1 := fmt.Sprintf(g.Config.UI.Front.MsgRemotePay+" "+g.Config.UI.Front.MsgPrice, currency.Amount(t.InMessage.ShowQR.DataInt).Format100I())
			g.Hardware.HD44780.Display.SetLines(l1, types.VMC.HW.Display.L2)
		}
	case tele_api.ShowQR_receipt:
		g.ShowQR(t.InMessage.ShowQR.QrText)
	case tele_api.ShowQR_error:
		g.ShowPicture(state.PictureQRPayError)
	case tele_api.ShowQR_errorOverdraft:
		g.UI().FrontSelectShowZero(ctx)
		g.ShowPicture(state.PicturePayReject)
	}

}
func (t *tele) checkCodePriceValid(menuCode *string, amount uint32) bool {
	i, found := types.UI.Menu[*menuCode]
	if !found {
		t.log.Infof("remote cook error: code not found")
		t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_executionInaccessible
		types.UI.FrontResult.Item = types.MenuItemType{}
		return false
	}
	if amount < uint32(i.Price) {
		t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_overdraft
		return false
	}
	if err := i.D.Validate(); err != nil {
		t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_executionInaccessible
		t.log.Infof("remote cook error: code not valid")
		return false
	}
	types.UI.FrontResult.Item = i
	return true
}

func (t *tele) dispatchCommand(ctx context.Context, cmd *tele_api.Command) error {
	switch task := cmd.Task.(type) {
	case *tele_api.Command_Report:
		return t.cmdReport(ctx, cmd)

	case *tele_api.Command_GetState:
		t.transport.SendState([]byte{byte(t.currentState)})
		return nil

	case *tele_api.Command_Exec:
		return t.cmdExec(ctx, cmd, task.Exec)

	case *tele_api.Command_SetInventory:
		return t.cmdSetInventory(ctx, cmd, task.SetInventory)

	case *tele_api.Command_ValidateCode:
		t.cmdValidateCode(cmd, task.ValidateCode.Code)
		return nil

	case *tele_api.Command_Cook:
		// return t.cmdCook(ctx, cmd, task.Cook)
		return nil

	case *tele_api.Command_Show_QR:
		return t.cmdShowQR(ctx, cmd, task.Show_QR)

	default:
		err := fmt.Errorf("unknown command=%#v", cmd)
		t.log.Error(err)
		return err
	}
}

func (t *tele) cmdValidateCode(cmd *tele_api.Command, code string) {
	mitem, ok := types.UI.Menu[code]
	if ok {
		if err := mitem.D.Validate(); err != nil {
			t.CookReply(cmd, tele_api.CookReplay_cookNothing, uint32(mitem.Price))
			return
		}
	}
	t.CookReply(cmd, tele_api.CookReplay_cookInaccessible)
}

func (t *tele) cmdReport(ctx context.Context, cmd *tele_api.Command) error {
	return errors.Annotate(t.Report(ctx, false), "cmdReport")
}

type FromRoboMessage tele_api.FromRoboMessage

func (t *tele) RemCook(ctx context.Context, o tele_api.FromRoboMessage) (err error) {
	types.UI.FrontResult.PaymenId = o.Order.OwnerInt
	err = ui.Cook(ctx)
	if err == nil {
		o.Order.OrderStatus = tele_api.OrderStatus_complete
		o.State = tele_api.State_Nominal
		types.VMC.UiState = 10 //FIXME StateFrontEnd     // 10 ->FrontBegin
	} else {
		o.State = tele_api.State_Broken
		if types.VMC.MonSys.Dirty != 0 {
			o.Order.OrderStatus = tele_api.OrderStatus_orderError
		}
		o.Err = &tele_api.Err{Message: err.Error()}
		types.VMC.UiState = 2 //FIXME  StateBroken
	}
	t.RoboSend(&o)
	return nil
}

// tunecook(value uint8, maximum uint8, defined uint8) (convertedvalue uint8)
// для робота занчения  от 0 (0=not change) до максимума. поэтому передаваемые значени = +1
func tuneCook(b []byte, def uint8, max uint8) uint8 {
	if len(b) == 0 {
		return def
	} else {
		v := b[0]
		if v == 0 {
			return def
		}
		if v > max+1 {
			return max
		}
		return v - 1
	}
}

func (t *tele) cmdExec(ctx context.Context, cmd *tele_api.Command, arg *tele_api.Command_ArgExec) error {
	if arg.Scenario[:1] == "_" { // If the command contains the "_" prefix, then you ignore the client lock flag
		arg.Scenario = arg.Scenario[1:]
	} else if types.VMC.Lock {
		t.log.Infof("ignore income remove command (locked) from: (%v) scenario: (%s)", cmd.Executer, arg.Scenario)
		t.CommandReply(cmd, tele_api.CmdReplay_busy)
		return errors.New("locked")
	}
	t.log.Infof("income remove command from: (%v) scenario: (%s)", cmd.Executer, arg.Scenario)
	g := state.GetGlobal(ctx)
	doer, err := g.Engine.ParseText("tele-exec", arg.Scenario)
	if err != nil {
		err = errors.Annotate(err, "parse")
		return err
	}
	err = doer.Validate()
	if err != nil {
		err = errors.Annotate(err, "validate")
		return err
	}

	if cmd.Executer > 0 {
		t.CommandReply(cmd, tele_api.CmdReplay_accepted)
	}

	err = g.ScheduleSync(ctx, doer.Do)
	if err == nil {
		t.CommandReply(cmd, tele_api.CmdReplay_done)
		return nil
	}
	t.CommandReply(cmd, tele_api.CmdReplay_error)
	err = errors.Annotate(err, "schedule")
	return err
}

func (t *tele) cmdSetInventory(ctx context.Context, cmd *tele_api.Command, arg *tele_api.Command_ArgSetInventory) error {
	if arg == nil || arg.New == nil {
		return errInvalidArg
	}

	g := state.GetGlobal(ctx)
	_, err := g.Inventory.SetTele(arg.New)
	return err
}

func (t *tele) cmdShowQR(ctx context.Context, cmd *tele_api.Command, arg *tele_api.Command_ArgShowQR) error {
	if arg == nil {
		return errInvalidArg
	}

	g := state.GetGlobal(ctx)
	g.ShowQR(arg.QrText)
	return nil
}
