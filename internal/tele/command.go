package tele

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/ui"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
	"google.golang.org/protobuf/proto"
)

var errInvalidArg = fmt.Errorf("invalid arg")

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
	m := tele_api.ToRoboMessage{}
	err := proto.Unmarshal(payload, &m)
	if err != nil {
		t.log.Errorf("tele command parse raw=%x err=%v", payload, err)
		return false
	}
	t.log.Infof("incoming message:%s", m.String())
	switch m.Cmd {
	case tele_api.MessageType_makeOrder:
		t.mesageMakeOrger(ctx, &m)
	case tele_api.MessageType_reportState:
		t.RoboSend(&tele_api.FromRoboMessage{
			State: t.currentState,
		})
	case tele_api.MessageType_reportStock:
	case tele_api.MessageType_showQR:
		t.messageShowQr(ctx, &m)
	case tele_api.MessageType_executeCommand:
	default: // unknow mesage type
	}
	return true
}

func (t *tele) mesageMakeOrger(ctx context.Context, m *tele_api.ToRoboMessage) {
	if m.MakeOrder == nil {
		return
	}

	currentRobotState := t.GetState()
	om := tele_api.FromRoboMessage{
		Order: &tele_api.Order{
			OwnerInt:  m.MakeOrder.OwnerInt,
			OwnerType: m.MakeOrder.OwnerType,
		},
	}
	rt := time.Now().Unix()
	st := m.ServerTime
	defer t.RoboSend(&om)
	if rt-st > 180 {
		// затычка по тайм ауту. если команда пришла с задержкой
		errM := fmt.Sprintf("remote make error. big time difference between server and robot. RTime:%v STime:%v", time.Unix(rt, 0), time.Unix(st, 0))
		t.log.Errorf(errM)
		om.Err = &tele_api.Err{Message: errM}
		om.Order.OrderStatus = tele_api.OrderStatus_orderError
		types.VMC.EvendKeyboardInput(true)
		return
	}
	switch m.MakeOrder.OrderStatus {
	case tele_api.OrderStatus_doSelected: // make selected code. payment via QR, etc
		if currentRobotState != tele_api.State_WaitingForExternalPayment || // robot state not wait
			types.UI.FrontResult.PaymenId != m.MakeOrder.OwnerInt { // the payer and payer do not match
			t.log.Errorf("make doSelected unposible. robo state:%s <> WaitingForExternalPayment or payerID:%d <> ownerID:%d", currentRobotState.String(), types.UI.FrontResult.PaymenId, m.MakeOrder.OwnerInt)
			om.Order.OrderStatus = tele_api.OrderStatus_orderError
			return
		}
		types.VMC.MonSys.Dirty = currency.Amount(m.MakeOrder.Amount)
	case tele_api.OrderStatus_doTransferred: // TODO execute external order. сделать внешний заказ
		// check validity, price. проверить валидность, цену
		if currentRobotState != tele_api.State_Nominal ||
			m.MakeOrder.PaymentMethod != tele_api.PaymentMethod_Balance {
			t.log.Errorf("make doTransferred unposible.robo state:%s <> Nominal or PaymentMethod %s <> Balance", currentRobotState.String(), tele_api.PaymentMethod_Balance.String())
			om.Order.OrderStatus = tele_api.OrderStatus_robotIsBusy
			return
		}
		i, found := types.UI.Menu[m.MakeOrder.MenuCode]
		if !found {
			t.log.Infof("remote cook error: code not found")
			om.Order.OrderStatus = tele_api.OrderStatus_executionInaccessible
			return
		}
		if err := i.D.Validate(); err != nil {
			om.Order.OrderStatus = tele_api.OrderStatus_executionInaccessible
			t.log.Infof("remote cook error: code not valid")
			return
		}
		types.UI.FrontResult.Item = i
		types.UI.FrontResult.Sugar = tuneCook(m.MakeOrder.GetSugar(), ui.DefaultSugar, ui.SugarMax())
		types.UI.FrontResult.Cream = tuneCook(m.MakeOrder.GetCream(), ui.DefaultCream, ui.CreamMax())
		om.Order.Amount = uint32(types.UI.FrontResult.Item.Price)
		types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price
		sound.PlayMoneyIn()
	default: // unknown status
		t.log.Errorf("unknown order status(%v)", m.MakeOrder.OrderStatus)
		om.Order.OrderStatus = tele_api.OrderStatus_orderError
		return
	}
	go func() { // run cooking
		remOr := tele_api.Order{
			Amount:        uint32(types.VMC.MonSys.Dirty),
			PaymentMethod: m.MakeOrder.PaymentMethod,
			OwnerInt:      m.MakeOrder.OwnerInt,
			OwnerType:     m.MakeOrder.OwnerType,
		}
		ui.OrderMenuAndTune(&remOr)
		g := state.GetGlobal(ctx)
		t.RemCook(ctx, &remOr)
		g.LockCh <- struct{}{} // дергаем state mashine
	}()
	om.State = tele_api.State_RemoteControl
	om.Order.OrderStatus = tele_api.OrderStatus_executionStart
}

func (t *tele) messageShowQr(ctx context.Context, m *tele_api.ToRoboMessage) {
	if m.ShowQR == nil {
		return
	}
	g := state.GetGlobal(ctx)
	switch m.ShowQR.QrType {
	case tele_api.ShowQR_order:
		if t.currentState == tele_api.State_WaitingForExternalPayment {
			// FIXME AlexM proto add QR payer and orderid
			// убрать услоивия когда все клиенты обносяться на верчию не менее 241207.0
			if m.ShowQR.Amount != 0 {
				types.UI.FrontResult.QRPayAmount = uint32(m.ShowQR.Amount)
			} else {
				types.UI.FrontResult.QRPayAmount = uint32(m.ShowQR.DataInt)
			}
			if m.ShowQR.PayerId != 0 {
				types.UI.FrontResult.PaymenId = m.ShowQR.PayerId
			} else {
				var err error
				types.UI.FrontResult.PaymenId, err = strconv.ParseInt(m.ShowQR.DataStr, 10, 64)
				if err != nil {
					t.Error(err)
				}
			}
			g.Log.Infof("show paymeng QR for order:%s", m.ShowQR.OrderId)
			g.ShowQR(m.ShowQR.QrText)
			l1 := fmt.Sprintf(g.Config.UI.Front.MsgRemotePay+g.Config.UI.Front.MsgPrice, currency.Amount(types.UI.FrontResult.QRPayAmount).Format100I())
			g.Hardware.HD44780.Display.SetLines(l1, types.VMC.HW.Display.L2)
		}
	case tele_api.ShowQR_receipt:
		t := m.ShowQR.QrText
		g.ShowQR(t)
	case tele_api.ShowQR_error:
		g.Hardware.Display.Graphic.CopyFile2FB(g.Config.UI.Front.PicQRPayError)
	case tele_api.ShowQR_errorOverdraft:
		g.Hardware.HD44780.Display.SetLines(g.Config.UI.Front.MsgMenuInsufficientCredit,
			g.Config.UI.Front.MsgRemotePayReject)
		g.Hardware.Display.Graphic.CopyFile2FB(g.Config.UI.Front.PicPayReject)
	}
}

func (t *tele) dispatchCommand(ctx context.Context, cmd *tele_api.Command) error {
	switch task := cmd.Task.(type) {
	case *tele_api.Command_Report:
		return t.cmdReport(ctx)

	case *tele_api.Command_GetState:
		t.transport.SendState([]byte{byte(t.currentState)})
		return nil

	case *tele_api.Command_Exec:
		return t.cmdExec(ctx, cmd, task.Exec)

	case *tele_api.Command_SetInventory:
		return t.cmdSetInventory(ctx, task.SetInventory)

	case *tele_api.Command_ValidateCode:
		t.cmdValidateCode(cmd, task.ValidateCode.Code)
		return nil

	case *tele_api.Command_Cook:
		// return t.cmdCook(ctx, cmd, task.Cook)
		return nil

	case *tele_api.Command_Show_QR:
		return t.cmdShowQR(ctx, task.Show_QR)

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

func (t *tele) cmdReport(ctx context.Context) error {
	return errors.Annotate(t.Report(ctx, false), "cmdReport")
}

type FromRoboMessage tele_api.FromRoboMessage

func (t *tele) RemCook(ctx context.Context, orderMake *tele_api.Order) (err error) {
	t.log.Infof("start remote cook order:%s ", orderMake.String())
	err = ui.Cook(ctx)
	if types.VMC.MonSys.Dirty == 0 {
		orderMake.OrderStatus = tele_api.OrderStatus_complete
	} else {
		orderMake.OrderStatus = tele_api.OrderStatus_orderError
	}
	rm := tele_api.FromRoboMessage{
		RoboTime: 0,
		Order:    orderMake,
	}
	if err == nil {
		rm.State = tele_api.State_Nominal
		types.VMC.UiState = uint32(types.StateFrontEnd)
	} else {
		rm.State = tele_api.State_Broken
		rm.Err = &tele_api.Err{Message: err.Error()}
		types.VMC.UiState = uint32(types.StateBroken)
	}
	t.log.Infof("order report:%s", rm.String())
	t.RoboSend(&rm)
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

func (t *tele) cmdSetInventory(ctx context.Context, arg *tele_api.Command_ArgSetInventory) error {
	if arg == nil || arg.New == nil {
		return errInvalidArg
	}

	g := state.GetGlobal(ctx)
	_, err := g.Inventory.SetTele(arg.New)
	return err
}

func (t *tele) cmdShowQR(ctx context.Context, arg *tele_api.Command_ArgShowQR) error {
	if arg == nil {
		return errInvalidArg
	}

	g := state.GetGlobal(ctx)
	g.ShowQR(arg.QrText)
	return nil
}
