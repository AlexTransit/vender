package money

import (
	"context"
	"time"

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
		g.Engine.Exec(ctx, ms.CoinValidator.AcceptMax(limit)),
	}
	err := helpers.FoldErrors(errs)
	if err != nil {
		err = oerr.Annotatef(err, "SetAcceptMax limit=%s", limit.FormatCtx(ctx))
	}
	return err
}

func (ms *MoneySystem) AcceptCredit(ctx context.Context, maxPrice currency.Amount, mainAlive *alive.Alive, out chan<- types.Event) error {
	g := state.GetGlobal(ctx)
	if ms.bill.GetState() != bill.Broken {
		go ms.bill.BillRun(mainAlive, func(e money.ValidatorEvent) {
			if e.Err != nil {
				ms.Log.Warning(e.Err)
				return
			}
			event := types.Event{}
			switch e.Event {
			case money.InEscrow:
				if (g.Config.Money.MinimalBill != 0 && g.Config.Money.MinimalBill > int(e.Nominal)) ||
					(g.Config.Money.MaximumBill != 0 && g.Config.Money.MaximumBill < int(e.Nominal)) {
					// say not posible
					// go sound.TextSpeech("купюру " + e.Nominal.Format100I() + " рублей не принимаем")
					// g.MustTextDisplay().SetLines("купюру "+e.Nominal.Format100I(), "не принимаем")
					ms.Log.Infof("reject money. min(%d) > (%s) > max(%d)", g.Config.Money.MinimalBill, e.Nominal.Format100I(), g.Config.Money.MaximumBill)
					ms.bill.SendCommand(bill.Reject)
					return
				}
				event.Kind = types.EventMoneyPreCredit
				ms.billCredit.Add(e.Nominal)
				if ms.GetCredit() < maxPrice {
					ms.bill.SendCommand(bill.Accept)
				}
			case money.OutEscrow:
				event.Kind = types.EventMoneyPreCredit
				if ms.billCredit.Total() > 0 {
					ms.billCredit.Sub(e.Nominal)
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
		if !ms.billReinited {
			ms.Log.Error("bill not work. state broken. reinit. send reset command")
			ms.billReinited = true
			go func() {
				ms.bill.BillReset()
				time.Sleep(40 * time.Second)
				if ms.bill.GetState() == bill.Broken {
					ms.Log.Error("bill not work after reset command.")
				}
			}()
		} else {
			ms.Log.Warning("bill not work")
		}
		mainAlive.Done()
	}
	// ----------------------coin ------------------------------------------------------------------
	go ms.CoinValidator.CoinRun(mainAlive, func(e money.ValidatorEvent) {
		event := types.Event{}
		switch e.Event {
		case money.CoinRejectKey:
			g.Hardware.Input.Emit(types.InputEvent{Source: input.MoneySourceTag, Key: input.MoneyKeyAbort})
		case money.CoinCredit:
			event.Kind = types.EventMoneyCredit
			ms.coinCredit.Add(e.Nominal)
			x := ms.GetCredit()
			if x >= maxPrice {
				ms.CoinValidator.DisableAccept()
			}
		default:
			ms.Log.WarningF("coin event not parce (%v)", e)
		}
		go func() { out <- event }()
		// out <- event
	})
	return nil
}
