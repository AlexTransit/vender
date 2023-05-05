package tele

import (
	"context"

	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	tele_api "github.com/AlexTransit/vender/tele"
)

const logMsgDisabled = "tele disabled"

func (t *tele) CommandReply(c *tele_api.Command, cr tele_api.CmdReplay) {
	if te := t.teleEnable(); te {
		return
	}
	t.log.Infof("command replay (%v) executer Id(%d)", cr, c.Executer)
	r := tele_api.Response{
		Executer:  c.Executer,
		CmdReplay: cr,
	}
	t.CommandResponse(&r)
}

func (t *tele) CookReply(c *tele_api.Command, cr tele_api.CookReplay, price ...uint32) {
	if te := t.teleEnable(); te {
		return
	}
	t.log.Infof("command replay (%v) executer Id(%d)", cr, c.Executer)
	r := tele_api.Response{
		Executer:   c.Executer,
		CookReplay: cr,
	}
	if price != nil {
		r.ValidateReplay = price[0]
	}
	t.CommandResponse(&r)
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
	if e == nil {
		t.log.Error("tele nil error")
		return
	}

	// t.log.Debugf("tele.Error: " + errors.ErrorStack(e))
	// alexm-comment duplicate
	// t.log.Err(errors.ErrorStack(e))
	t.ErrorStr(e.Error())
}

func (t *tele) Log(s string) {
	t.log.Infof(s)
}

func (t *tele) ErrorStr(s string) {
	tm := &tele_api.FromRoboMessage{
		Err: &tele_api.Err{
			Message: s,
		},
	}
	t.RoboSend(tm)
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
	}
	t.Telemetry(tm)
	return nil
}

// func (t *tele) State(s tele_api.State) {
// 	if t.currentState != s {
// 		t.currentState = s
// 		m := tele_api.FromRoboMessage{
// 			State:                s,
// 		}
// 		t.RoboSend(&m)
// 	}
// }

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
	t.Telemetry(&tele_api.Telemetry{Transaction: tx})
}
