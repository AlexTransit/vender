package money

import (
	"context"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/mdb/bill"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
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
	g := state.GetGlobal(ctx)
	// coinmax := currency.Amount(1000)
	// g.Engine.Exec(ctx, ms.coin.AcceptMax(coinmax))
	var stopAccept <-chan struct{}
	if mainAlive != nil {
		stopAccept = mainAlive.StopChan()
	}
	if ms.bill.GetState() != bill.Broken {
		go ms.bill.BillRun(mainAlive, func(e money.ValidatorEvent) {
			if e.Err != nil {
				ms.Log.Warning(e.Err)
				return
			}
			event := types.Event{}
			switch e.Event {
			case money.InEscrow:
				event.Kind = types.EventMoneyPreCredit
				if be.BillNominal <= currency.Nominal(g.Config.Money.CreditMax) {
					ms.billCredit.Add(be.BillNominal)
					if ms.GetCredit() < maxPrice {
						ms.bill.SendCommand(bill.Accept)
					}
				} else {
					ms.Log.Infof("reject big money (%v)", be.BillNominal.Format100I())
					ms.bill.SendCommand(bill.Reject)
					return
				}
			case money.OutEscrow:
				event.Kind = types.EventMoneyPreCredit
				if ms.billCredit.Total() > 0 {
					ms.billCredit.Sub(be.BillNominal)
				}
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
		mainAlive.Done()
	}
	// ----------------------coin ------------------------------------------------------------------
	go ms.coin.CoinRun(mainAlive, func(e money.ValidatorEvent) {
		event := types.Event{}
		switch e.Event {
		case money.CoinRejectKey:
			g.Hardware.Input.Emit(types.InputEvent{Source: input.MoneySourceTag, Key: input.MoneyKeyAbort})
		case money.CoinCredit:
			event.Kind = types.EventMoneyCredit
			ms.coinCredit.Add(e.Nominal)
			x := ms.GetCredit()
			if x >= maxPrice {
				ms.coin.DisableAccept()
			}
		default:
		}
		go func() { out <- event }()
		// out <- event
	})
	<-stopAccept
	ms.bill.SendCommand(bill.Stop)
	return nil
}
