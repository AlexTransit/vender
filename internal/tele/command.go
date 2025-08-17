package tele

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/AlexTransit/vender/currency"
	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
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
		go t.mesageMakeOrger(ctx, &m)
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
	g := state.GetGlobal(ctx)
	g.UI().PauseStateMashine(true)
	defer g.UI().PauseStateMashine(false)
	currentRobotState := t.GetState()
	switch currentRobotState {
	case tele_api.State_Nominal, tele_api.State_WaitingForExternalPayment:
		rt := time.Now().Unix()
		st := m.ServerTime
		if rt-st > 180 {
			// затычка по тайм ауту. если команда пришла с задержкой
			errM := fmt.Sprintf("remote make error. big time difference between server and robot. RTime:%v STime:%v", time.Unix(rt, 0), time.Unix(st, 0))
			t.log.Error(errM)
			t.makeOrderImposible(tele_api.OrderStatus_orderError, m)
			return
		}
	default: // state not valid
		return
	}
	u := config_global.VMC.User
	switch m.MakeOrder.OrderStatus {
	case tele_api.OrderStatus_doSelected: // make selected code. payment via QR, etc
		if config_global.VMC.User.PaymenId != m.MakeOrder.OwnerInt || // the payer and payer do not match
			uint32(config_global.VMC.User.DirtyMoney) != m.MakeOrder.Amount { //
			t.log.Errorf("make doSelected unposible. robo state:%s <> WaitingForExternalPayment or payerID:%d <> ownerID:%d or qr amount:%d<>order amount^%d",
				currentRobotState.String(), config_global.VMC.User.PaymenId, m.MakeOrder.OwnerInt, config_global.VMC.User.DirtyMoney, m.MakeOrder.Amount)
			t.makeOrderImposible(tele_api.OrderStatus_orderError, m)
			return
		}
		// config_global.VMC.User.SelectedItem.Price = config_global.VMC.User.DirtyMoney
		u.SelectedItem.Price = config_global.VMC.User.DirtyMoney
	case tele_api.OrderStatus_doTransferred: // TODO execute external order. сделать внешний заказ
		if m.MakeOrder.PaymentMethod != tele_api.PaymentMethod_Balance {
			t.log.Errorf("make doTransferred unposible.robo state:%s <> Nominal or PaymentMethod %s <> Balance", currentRobotState.String(), tele_api.PaymentMethod_Balance.String())
			t.makeOrderImposible(tele_api.OrderStatus_robotIsBusy, m)
			return
		}
	default: // unknown status
		t.log.Errorf("unknown order status(%v)", m.MakeOrder.OrderStatus)
		return
	}
	var found bool
	u.SelectedItem, found = config_global.GetMenuItem(m.MakeOrder.MenuCode)
	if !found {
		t.log.Infof("remote cook error: code not found")
		t.makeOrderImposible(tele_api.OrderStatus_executionInaccessible, m)
		return
	}
	if err := u.SelectedItem.Doer.Validate(); err != nil {
		t.makeOrderImposible(tele_api.OrderStatus_executionInaccessible, m)
		t.log.Infof("remote cook error: code not valid")
		return
	}
	sound.PlayMoneyIn()
	u.Sugar = tuneCook(m.MakeOrder.GetSugar(), config_global.VMC.Engine.Menu.DefaultSugar, config_global.VMC.Engine.Menu.DefaultSugarMax)
	u.Cream = tuneCook(m.MakeOrder.GetCream(), config_global.VMC.Engine.Menu.DefaultCream, config_global.VMC.Engine.Menu.DefaultCreamMax)
	u.DirtyMoney = u.SelectedItem.Price
	u.PaymenId = m.MakeOrder.OwnerInt
	u.PaymentMethod = m.MakeOrder.PaymentMethod
	u.PaymentType = m.MakeOrder.OwnerType
	ms := money.GetGlobal(ctx)
	ms.SetDirty(config_global.VMC.User.DirtyMoney)
	config_global.VMC.User = u
	// run cooking
	g.UI().CreateEvent(types.EventAccept)
}

func (t *tele) makeOrderImposible(oStatus tele_api.OrderStatus, m *tele_api.ToRoboMessage) {
	rm := tele_api.FromRoboMessage{
		State: t.currentState,
		Order: &tele_api.Order{
			OrderStatus:   oStatus,
			PaymentMethod: 0,
			OwnerInt:      m.MakeOrder.OwnerInt,
			OwnerType:     m.MakeOrder.OwnerType,
		},
	}
	t.RoboSend(&rm)
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
			// убрать услоивия когда все клиенты обновяться на верчию не менее 241207.0
			if m.ShowQR.Amount != 0 {
				config_global.VMC.User.QRPayAmount = uint32(m.ShowQR.Amount)
			} else {
				config_global.VMC.User.QRPayAmount = uint32(m.ShowQR.DataInt)
			}
			if m.ShowQR.PayerId != 0 {
				config_global.VMC.User.PaymenId = m.ShowQR.PayerId
			} else {
				var err error
				config_global.VMC.User.PaymenId, err = strconv.ParseInt(m.ShowQR.DataStr, 10, 64)
				if err != nil {
					t.Error(err)
				}
			}
			g.Log.Infof("show paymeng QR for order:%s", m.ShowQR.OrderId)
			g.ShowQR(m.ShowQR.QrText)
			l1 := fmt.Sprintf(g.Config.UI_config.Front.MsgRemotePay+g.Config.UI_config.Front.MsgPrice, currency.Amount(config_global.VMC.User.QRPayAmount).Format100I())
			g.Hardware.HD44780.Display.SetLine(1, l1)
			config_global.VMC.User.DirtyMoney = currency.Amount(config_global.VMC.User.QRPayAmount)
			config_global.VMC.User.PaymentType = tele_api.OwnerType_qrCashLessUser
			config_global.VMC.User.PaymentMethod = tele_api.PaymentMethod_Cashless
		}
	case tele_api.ShowQR_receipt:
		t := m.ShowQR.QrText
		g.ShowQR(t)
	case tele_api.ShowQR_error:
		g.Hardware.Display.Graphic.CopyFile2FB(g.Config.UI_config.Front.PicQRPayError)
	case tele_api.ShowQR_errorOverdraft:
		g.Hardware.HD44780.Display.SetLines(g.Config.UI_config.Front.MsgMenuInsufficientCreditL1,
			g.Config.UI_config.Front.MsgRemotePayReject)
		g.Hardware.Display.Graphic.CopyFile2FB(g.Config.UI_config.Front.PicPayReject)
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
		// return t.cmdSetInventory(ctx, task.SetInventory)
		return nil

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
	mitem, ok := config_global.GetMenuItem(code)
	if ok {
		if err := mitem.Doer.Validate(); err != nil {
			t.CookReply(cmd, tele_api.CookReplay_cookNothing, uint32(mitem.Price))
			return
		}
	}
	t.CookReply(cmd, tele_api.CookReplay_cookInaccessible)
}

func (t *tele) cmdReport(ctx context.Context) error {
	return errors.Annotate(t.Report(ctx, false), "cmdReport")
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
	} else if config_global.VMC.User.Lock {
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

// func (t *tele) cmdSetInventory(ctx context.Context, arg *tele_api.Command_ArgSetInventory) error {
// 	if arg == nil || arg.New == nil {
// 		return errInvalidArg
// 	}

// 	g := state.GetGlobal(ctx)
// 	_, err := g.Inventory.SetTele(arg.New)
// 	return err
// }

func (t *tele) cmdShowQR(ctx context.Context, arg *tele_api.Command_ArgShowQR) error {
	if arg == nil {
		return errInvalidArg
	}

	g := state.GetGlobal(ctx)
	g.ShowQR(arg.QrText)
	return nil
}
