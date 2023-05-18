package money

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb/bill"
	"github.com/AlexTransit/vender/hardware/money"

	"github.com/AlexTransit/vender/hardware/input"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"

	tele_api "github.com/AlexTransit/vender/tele"
	oerr "github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

func (ms *MoneySystem) SetAcceptMax(ctx context.Context, limit currency.Amount) error {
	g := state.GetGlobal(ctx)
	errs := []error{
		// g.Engine.Exec(ctx, ms.bill.AcceptMax(limit)),
		g.Engine.Exec(ctx, ms.coin.AcceptMax(limit)),
	}
	err := helpers.FoldErrors(errs)
	if err != nil {
		err = oerr.Annotatef(err, "SetAcceptMax limit=%s", limit.FormatCtx(ctx))
	}
	return err
}

func (ms *MoneySystem) AcceptCredit(ctx context.Context, maxPrice currency.Amount, mainAlive *alive.Alive, out chan<- types.Event) error {
	const tag = "money.accept-credit"
	g := state.GetGlobal(ctx)

	coinmax := currency.Amount(1000)

	g.Engine.Exec(ctx, ms.coin.AcceptMax(coinmax))
	stopAccept := mainAlive.StopChan()
	validatorAlive := alive.NewAlive()
	validatorAlive.Add(1)
	if ms.bill.GetState() != bill.Broken {
		go ms.bill.BillRun(validatorAlive, func(e money.ValidatorEvent) {
			if e.Err != nil {
				ms.Log.Warning(e.Err)
				return
			}
			event := types.Event{}
			switch e.Event {
			case money.InEscrow:
				event.Kind = types.EventMoneyPreCredit
				ms.billCredit.Add(e.Nominal)
				if ms.GetCredit() < maxPrice {
					ms.bill.SendCommand(bill.Accept)
				}
			case money.OutEscrow:
				event.Kind = types.EventMoneyPreCredit
				ms.billCredit.Sub(e.Nominal)
			case money.Stacked:
				event.Kind = types.EventMoneyCredit
				if !ms.bill.BillStacked() {
					ms.Log.Error("bill not stacked. substruct bill credit")
					ms.billCredit.Sub(e.Nominal)
				}
			default:
				return
			}
			go func() { out <- event }()
		})
	} else {
		ms.Log.Warning("bill not work")
		validatorAlive.Done()
	}

	// ----------------------coin ------------------------------------------------------------------
	go ms.coin.Run(ctx, mainAlive, func(pi money.PollItem) bool {
		ms.lk.Lock()
		defer ms.lk.Unlock()
		switch pi.Status {
		case money.StatusDispensed:
			ms.Log.Debugf("%s manual dispense: %s", tag, pi.String())
			_ = ms.coin.TubeStatus()
			_ = ms.coin.ExpansionDiagStatus(nil)
		case money.StatusReturnRequest:
			// XXX maybe this should be in coin driver
			g.Hardware.Input.Emit(types.InputEvent{Source: input.MoneySourceTag, Key: input.MoneyKeyAbort})
		case money.StatusRejected:
			g.Tele.StatModify(func(s *tele_api.Stat) {
				s.CoinRejected[uint32(pi.DataNominal)] += uint32(pi.DataCount)
			})
		case money.StatusCredit:
			g.ClientBegin(ctx)
			if pi.DataCashbox {
				if err := ms.coinCashbox.AddMany(pi.DataNominal, uint(pi.DataCount)); err != nil {
					g.Error(oerr.Annotatef(err, "%s cashbox.Add n=%v c=%d", tag, pi.DataNominal, pi.DataCount))
					break
				}
			}
			err := ms.coinCredit.AddMany(pi.DataNominal, uint(pi.DataCount))
			if err != nil {
				g.Error(oerr.Annotatef(err, "%s credit.Add n=%v c=%d", tag, pi.DataNominal, pi.DataCount))
				break
			}
			_ = ms.coin.TubeStatus()
			_ = ms.coin.ExpansionDiagStatus(nil)
			// ms.dirty += pi.Amount()
			ms.AddDirty(pi.Amount())
			ms.Log.Infof("coin accepted:%v", pi.Amount().Format100I())
			// validatorAlive.Done()
			if out != nil {
				event := types.Event{Kind: types.EventMoneyCredit, Amount: pi.Amount()}
				// async channel send to avoid deadlock lk.Lock vs <-out
				go func() { out <- event }()
			}
		default:
			g.Error(fmt.Errorf("CRITICAL code error unhandled coin POLL item=%#v", pi))
		}
		return false
	})
	//
	again := true
	for again {
		select {
		case <-validatorAlive.WaitChan():
			again = false
		case <-stopAccept:
			validatorAlive.Stop()
			ms.lk.Lock()
			defer ms.lk.Unlock()
			ms.bill.SendCommand(bill.Stop)
			again = false
		}
	}
	validatorAlive.Wait()
	return nil
}
