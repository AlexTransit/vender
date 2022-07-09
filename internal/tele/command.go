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
		t.CommandReplyErr(cmd, err)
		t.log.Errorf("command message error (%v)", err)
		return true
	}

	return true
}

func (t *tele) messageForRobot(ctx context.Context, payload []byte) bool {
	im := new(tele_api.ToRoboMessage) // input message
	err := proto.Unmarshal(payload, im)
	if err != nil {
		t.log.Errorf("tele command parse raw=%x err=%v", payload, err)
		return false
	}
	t.log.Infof("incoming message:%v", im)
	g := state.GetGlobal(ctx)
	if im.Cmd == tele_api.MessageType_reportState {
		t.RoboSend(&tele_api.FromRoboMessage{
			State: t.currentState,
		})
	}
	if im.MakeOrder != nil {
		// prepare output message
		t.OutMessage = tele_api.FromRoboMessage{
			Order: &tele_api.Order{},
		}
		t.OutMessage.Order.OwnerInt = im.MakeOrder.OwnerInt
		t.OutMessage.Order.OwnerStr = im.MakeOrder.OwnerStr
		t.OutMessage.Order.OwnerType = im.MakeOrder.OwnerType
		defer t.RoboSend(&t.OutMessage)
		rt := time.Now().Unix()
		st := im.ServerTime
		if rt-st > 180 {
			// AlexM FIXME затычка по тайм ауту.
			errM := fmt.Sprintf("remote make error. big time difference between server and robot. RTime:%v STime:%v", time.Unix(rt, 0), time.Unix(st, 0))
			t.OutMessage.Err = &tele_api.Err{Message: errM}
			t.log.Errorf(errM)
			t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_orderError
			types.VMC.EvendKeyboardInput(true)
			l1 := g.Config.UI.Front.MsgMenuInsufficientCreditL1
			l2 := fmt.Sprintf(g.Config.UI.Front.MsgMenuInsufficientCreditL2, "0", types.UI.FrontResult.Item.Price.Format100I())
			g.Hardware.HD44780.Display.SetLines(l1, l2)
			g.ShowPicture(state.PictureClient)
			return false
		}

		switch im.MakeOrder.OrderStatus {
		case tele_api.OrderStatus_doSelected:
			// make selected code. payment via QR, etc
			if !types.UI.FrontResult.Accepted || t.currentState != tele_api.State_WaitingForExternalPayment {
				t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_robotIsBusy
				return false
			}
			t.reportExecutionStart()
			types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price
			t.OutMessage.Order.PaymentMethod = tele_api.PaymentMethod_Cashless
			t.RemCook(ctx)
			g.LockCh <- struct{}{}

		case tele_api.OrderStatus_doTransferred:
			//TODO execute external order. сделать внешний заказ
			// check validity, price. проверить валидность, цену
			if t.currentState != tele_api.State_Nominal {
				t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_robotIsBusy
				return false
			}
			// when paying by balance, the current balance of the client is sent. при оплате по балансу, присылвется текущий баланс клиента
			if im.MakeOrder.PaymentMethod != tele_api.PaymentMethod_Balance {
				t.OutMessage.Err.Message = "command remote cook, not set balance"
				return false
			}
			var found bool
			types.UI.FrontResult.Item, found = types.UI.Menu[im.MakeOrder.MenuCode]
			if !found {
				t.log.Infof("remote cook error: code not found")
				t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_executionInaccessible
				return false
			}
			price := uint32(types.UI.FrontResult.Item.Price)
			if price > im.MakeOrder.Amount {
				t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_overdraft
				return false
			}
			if err := types.UI.FrontResult.Item.D.Validate(); err != nil {
				t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_executionInaccessible
				t.log.Infof("remote cook error: code not valid")
				return false
			}
			t.reportExecutionStart()
			types.UI.FrontResult.Sugar = tuneCook(im.MakeOrder.Sugar, ui.DefaultSugar, ui.MaxSugar)
			types.UI.FrontResult.Cream = tuneCook(im.MakeOrder.Cream, ui.DefaultCream, ui.MaxCream)
			t.OutMessage.Order.Amount = price
			types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price
			t.RemCook(ctx)
			g.LockCh <- struct{}{}
		}

	}

	if im.ShowQR != nil {
		switch im.ShowQR.QrType {
		case tele_api.ShowQR_order:
			if types.UI.FrontResult.Accepted {
				g.ShowQR(im.ShowQR.QrText)
				l1 := fmt.Sprintf(g.Config.UI.Front.MsgRemotePayL1, currency.Amount(im.ShowQR.DataInt).Format100I())
				g.Hardware.HD44780.Display.SetLines(l1, types.VMC.HW.Display.L2)
			}
		case tele_api.ShowQR_receipt:
			g.ShowQR(im.ShowQR.QrText)
		case tele_api.ShowQR_error:
			g.ShowPicture(state.PictureQRPayError)
		case tele_api.ShowQR_errorOverdraft:
			g.ShowPicture(state.PictureQRPayError)
		}
	}
	return true
}

func (t *tele) reportExecutionStart() {
	t.OutMessage.State = tele_api.State_RemoteControl
	t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_executionStart
	t.RoboSend(&t.OutMessage)
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

func (t *tele) cmdCook(ctx context.Context, cmd *tele_api.Command, arg *tele_api.Command_ArgCook) error {
	if types.VMC.UiState != uint32(ui.StatePrepare) {
		if types.VMC.Lock {
			t.log.Infof("ignore remote make command (locked) from: (%v) scenario: (%s)", cmd.Executer, arg.Menucode)
			t.CookReply(cmd, tele_api.CookReplay_vmcbusy)
			return nil
		}
		var checkVal bool
		types.UI.FrontResult.Item, checkVal = types.UI.Menu[arg.Menucode]
		if !checkVal {
			t.CookReply(cmd, tele_api.CookReplay_cookInaccessible)
			t.log.Infof("remote cook error: code not found")
			return nil
		}
		if err := types.UI.FrontResult.Item.D.Validate(); err != nil {
			t.CookReply(cmd, tele_api.CookReplay_cookInaccessible)
			t.log.Infof("remote cook error: code not valid")
			return nil
		}
		types.UI.FrontResult.Sugar = tuneCook(arg.Sugar, ui.DefaultSugar, ui.MaxSugar)
		types.UI.FrontResult.Cream = tuneCook(arg.Cream, ui.DefaultCream, ui.MaxCream)
		types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price

		t.CookReply(cmd, tele_api.CookReplay_cookStart)
		t.log.Infof("remote coocing (%v) (%v)", cmd, arg)
	} else if arg.Menucode != "-" {
		t.log.Infof("ignore remote make command (locked) from: (%v) scenario: (%s)", cmd.Executer, arg.Menucode)
		t.CookReply(cmd, tele_api.CookReplay_vmcbusy)
		return nil
	}
	t.RoboSendState(tele_api.State_RemoteControl)

	if arg.Balance < int32(types.UI.FrontResult.Item.Price) {
		t.CookReply(cmd, tele_api.CookReplay_cookOverdraft, uint32(types.UI.FrontResult.Item.Price))
		t.log.Infof("remote cook inposible. ovedraft. balance=%d price=%d", arg.Balance, types.UI.FrontResult.Item.Price)
		return nil
	}
	types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price
	err := ui.Cook(ctx)
	if types.VMC.MonSys.Dirty == 0 {
		rm := tele_api.Response{
			Executer:       cmd.Executer,
			CookReplay:     tele_api.CookReplay_cookFinish,
			ValidateReplay: uint32(types.UI.FrontResult.Item.Price),
		}
		if arg.Menucode != types.UI.FrontResult.Item.Code {
			rm.Data = types.UI.FrontResult.Item.Code
		}
		t.CommandResponse(&rm)
	}
	if err != nil {
		t.CookReply(cmd, tele_api.CookReplay_cookError)
		t.RoboSendState(tele_api.State_Broken)
		types.VMC.UiState = uint32(ui.StateBroken)
		return errors.Errorf("remote cook make error: (%v)", err)
	}
	t.RoboSendState(tele_api.State_Nominal)
	state.VmcUnLock(ctx)
	return nil
}

type FromRoboMessage tele_api.FromRoboMessage

func (t *tele) RemCook(ctx context.Context) (err error) {
	t.OutMessage.Order.MenuCode = types.UI.FrontResult.Item.Code
	t.OutMessage.Order.Cream = types.TuneValueToByte(types.UI.FrontResult.Cream, ui.DefaultCream)
	t.OutMessage.Order.Sugar = types.TuneValueToByte(types.UI.FrontResult.Sugar, ui.DefaultSugar)

	err = ui.Cook(ctx)
	if err == nil {
		t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_complete
		t.OutMessage.State = tele_api.State_Nominal
		types.VMC.UiState = 10 //FIXME StateFrontEnd     // 10 ->FrontBegin
	} else {
		t.OutMessage.State = tele_api.State_Broken
		if types.VMC.MonSys.Dirty != 0 {
			t.OutMessage.Order.OrderStatus = tele_api.OrderStatus_orderError
		}
		t.OutMessage.Err = &tele_api.Err{Message: err.Error()}
		types.VMC.UiState = 2 //FIXME  StateBroken
	}

	return nil
}

func tuneCook(b []byte, def uint8, max uint8) uint8 {
	// tunecook(value uint8, maximum uint8, defined uint8) (convertedvalue uint8)
	// для робота занчения  от 0 (0=not change) до максимума. поэтому передаваемые значени = +1
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
