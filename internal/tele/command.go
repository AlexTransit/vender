package tele

import (
	"context"
	"fmt"

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
		// TODO reply error
		return true
	}
	g := state.GetGlobal(ctx)

	if im.MakeOrder != nil {
		// проверить время валидности QR и цену
		om := tele_api.FromRoboMessage{
			RobotState: &tele_api.RobotState{
				State: tele_api.State_Process,
			},
			Order: &tele_api.Order{
				OrderStatus: tele_api.OrderStatus_executionStart,
				OwnerInt:    im.MakeOrder.OwnerInt,
			},
		}
		t.RoboSend(&om)

	}

	if im.ShowQR != nil {
		t.log.Infof("input meggase ShowQr. type:%v message:%s", im.ShowQR.QrType, im.ShowQR.QrText)
		g.ShowQR(im.ShowQR.QrText)
	}
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
		return t.cmdCook(ctx, cmd, task.Cook)

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
	if types.VMC.State != int32(ui.StatePrepare) {
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
		t.State(tele_api.State_RemoteControl)
	} else if arg.Menucode != "-" {
		t.log.Infof("ignore remote make command (locked) from: (%v) scenario: (%s)", cmd.Executer, arg.Menucode)
		t.CookReply(cmd, tele_api.CookReplay_vmcbusy)
		return nil
	}

	if arg.Balance < int32(types.UI.FrontResult.Item.Price) {
		t.CookReply(cmd, tele_api.CookReplay_cookOverdraft, uint32(types.UI.FrontResult.Item.Price))
		t.log.Infof("remote cook inposible. ovedraft. balance=%d price=%d", arg.Balance, types.UI.FrontResult.Item.Price)
		return nil
	}
	types.VMC.MonSys.Dirty = types.UI.FrontResult.Item.Price
	err := ui.Cook(ctx)
	if types.VMC.MonSys.Dirty == 0 {
		// t.CookReply(cmd, tele_api.CookReplay_cookFinish, uint32(types.UI.FrontResult.Item.Price))
		// r.ValidateReplay = price[0]
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
		t.State(tele_api.State_Broken)
		types.VMC.State = int32(ui.StateBroken)
		return errors.Errorf("remote cook make error: (%v)", err)
	}
	t.State(tele_api.State_Nominal)
	state.VmcUnLock(ctx)
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
	// display, err := g.Display()
	// if err != nil {
	// 	return errors.Annotate(err, "display")
	// }
	// if display == nil {
	// 	return fmt.Errorf("display is not configured")
	// }
	// // TODO display.Layout(arg.Layout)
	// // TODO border,redundancy from layout/config
	// t.log.Infof("show QR:'%v'", arg.QrText)
	// types.VMC.HW.Display.Gdisplay = arg.QrText
	// return display.QR(arg.QrText, true, qrcode.High)
	return nil
}

// -------------------
func (t *tele) inputRoboMessage(ctx context.Context, payload []byte) {
	t.log.Infof("\n\033[41m sdf \033[0m\n\n")
}
