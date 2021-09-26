package tele

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
)

const logMsgDisabled = "tele disabled"

func (t *tele) CommandReplyErr(c *tele_api.Command, e error) {
	if !t.config.Enabled {
		t.log.Infof(logMsgDisabled)
		return
	}
	errText := ""
	if e != nil {
		errText = e.Error()
	}
	r := tele_api.Response{
		// CommandId: c.Id,
		Error: errText,
	}
	err := t.qpushCommandResponse(c, &r)
	if err != nil {
		t.log.Error(errors.Annotatef(err, "CRITICAL command=%#v response=%#v", c, r))
	}
}

func (t *tele) CommandReply(c *tele_api.Command, cr tele_api.CmdReplay) {
	if te := t.teleEnable(); te {
		return
	}
	t.log.Infof("command replay (%v) executer Id(%d)", cr, c.Executer)
	r := tele_api.Response{
		Executer:  c.Executer,
		CmdReplay: cr,
	}
	err := t.qpushCommandResponse(c, &r)
	if err != nil {
		// s.log.Error(errors.Annotatef(err, "CRITICAL command=%#v response=%#v", c, r))
		fmt.Printf("\n\033[41m error \033[0m\n\n")
	}
}

func (t *tele) CookReply(c *tele_api.Command, cr tele_api.CookReplay) {
	if te := t.teleEnable(); te {
		return
	}
	t.log.Infof("command replay (%v) executer Id(%d)", cr, c.Executer)
	r := tele_api.Response{
		Executer:   c.Executer,
		CookReplay: cr,
	}
	err := t.qpushCommandResponse(c, &r)
	if err != nil {
		fmt.Printf("\n\033[41m error \033[0m\n\n")
	}
}

func (t *tele) teleEnable() bool {
	if !t.config.Enabled {
		t.log.Infof(logMsgDisabled)
		return true
	}
	return false
}

func (t *tele) Error(e error) {
	if !t.config.Enabled {
		t.log.Infof(logMsgDisabled)
		return
	}

	t.log.Debugf("tele.Error: " + errors.ErrorStack(e))
	tm := &tele_api.Telemetry{
		Error: &tele_api.Telemetry_Error{Message: e.Error()},
		// BuildVersion: t.config.BuildVersion,
	}
	if err := t.qpushTelemetry(tm); err != nil {
		t.log.Errorf("CRITICAL qpushTelemetry telemetry_error=%#v err=%v", tm.Error, err)
	}
}

func (t *tele) Report(ctx context.Context, serviceTag bool) error {
	if !t.config.Enabled {
		t.log.Infof(logMsgDisabled)
		return nil
	}

	g := state.GetGlobal(ctx)
	moneysys := money.GetGlobal(ctx)
	tm := &tele_api.Telemetry{
		Inventory:    g.Inventory.Tele(),
		MoneyCashbox: moneysys.TeleCashbox(ctx),
		MoneyChange:  moneysys.TeleChange(ctx),
		AtService:    serviceTag,
		// BuildVersion: g.BuildVersion,
	}
	err := t.qpushTelemetry(tm)
	if err != nil {
		t.log.Errorf("CRITICAL qpushTelemetry tm=%#v err=%v", tm, err)
	}
	return err
}

func (t *tele) State(s tele_api.State) {
	if t.currentState != s {
		t.currentState = s
		t.transport.SendState([]byte{byte(s)})
	}
}

func (t *tele) StatModify(fun func(s *tele_api.Stat)) {
	if !t.config.Enabled {
		t.log.Infof(logMsgDisabled)
		return
	}

	t.stat.Lock()
	fun(&t.stat)
	t.stat.Unlock()
}

func (t *tele) Transaction(tx *tele_api.Telemetry_Transaction) {
	if !t.config.Enabled {
		t.log.Infof(logMsgDisabled)
		return
	}
	err := t.qpushTelemetry(&tele_api.Telemetry{Transaction: tx})
	if err != nil {
		t.log.Errorf("CRITICAL transaction=%#v err=%v", tx, err)
	}
}
