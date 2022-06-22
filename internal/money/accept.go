package money

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

func (ms *MoneySystem) SetAcceptMax(ctx context.Context, limit currency.Amount) error {
	g := state.GetGlobal(ctx)
	errs := []error{
		g.Engine.Exec(ctx, ms.bill.AcceptMax(limit)),
		g.Engine.Exec(ctx, ms.coin.AcceptMax(limit)),
	}
	err := helpers.FoldErrors(errs)
	if err != nil {
		err = errors.Annotatef(err, "SetAcceptMax limit=%s", limit.FormatCtx(ctx))
	}
	return err
}

func (ms *MoneySystem) AcceptCredit(ctx context.Context, maxPrice currency.Amount, stopAccept <-chan struct{}, out chan<- types.Event) error {
	const tag = "money.accept-credit"

	g := state.GetGlobal(ctx)

	maxConfig := currency.Amount(g.Config.Money.CreditMax)
	// Accept limit = lesser of: configured max credit or highest menu price.
	limit := maxConfig
	billmax := maxConfig
	coinmax := currency.Amount(1000)

	ms.lk.Lock()
	available := ms.locked_credit(creditCash | creditEscrow)
	ms.lk.Unlock()
	if available != 0 && limit >= available {
		limit -= available
	}
	if available >= maxPrice {
		limit = 0
		billmax = 0
		coinmax = 0
		ms.Log.Debugf("%s money input disable", tag)
	}
	// if ms.Credit(ctx) != 0 {
	// 	ms.Log.Debugf("%s maxConfig=%s maxPrice=%s available=%s -> limit=%s",
	// 		tag, maxConfig.FormatCtx(ctx), maxPrice.FormatCtx(ctx), available.FormatCtx(ctx), limit.FormatCtx(ctx))
	// }

	g.Engine.Exec(ctx, ms.bill.AcceptMax(billmax))
	g.Engine.Exec(ctx, ms.coin.AcceptMax(coinmax))
	// err := ms.SetAcceptMax(ctx, limit)
	// if err != nil {
	// 	return err
	// }

	alive := alive.NewAlive()
	alive.Add(2)
	if billmax != 0 {
		go ms.bill.Run(ctx, alive, func(pi money.PollItem) bool {
			g.ClientBegin()
			switch pi.Status {
			case money.StatusEscrow:
				if ms.bill.EscrowAmount() == 0 {
					g.Log.Error("ERR status escrow when there is no money in escrow!")
				}
				err := g.Engine.Exec(ctx, ms.bill.EscrowAccept())
				if err != nil {
					g.Error(errors.Annotatef(err, "money.bill escrow accept n=%s", currency.Amount(pi.DataNominal).FormatCtx(ctx)))
				}
			case money.StatusCredit:
				ms.lk.Lock()
				defer ms.lk.Unlock()

				if pi.DataCashbox {
					if err := ms.billCashbox.Add(pi.DataNominal, uint(pi.DataCount)); err != nil {
						g.Error(errors.Annotatef(err, "money.bill cashbox.Add n=%v c=%d", pi.DataNominal, pi.DataCount))
						break
					}
				}
				if err := ms.billCredit.Add(pi.DataNominal, uint(pi.DataCount)); err != nil {
					g.Error(errors.Annotatef(err, "money.bill credit.Add n=%v c=%d", pi.DataNominal, pi.DataCount))
					break
				}
				// ms.Log.Debugf("money.bill credit amount=%s bill=%s cash=%s total=%s",
				// 	pi.Amount().FormatCtx(ctx), ms.billCredit.Total().FormatCtx(ctx),
				// 	ms.locked_credit(creditCash|creditEscrow).FormatCtx(ctx),
				// 	ms.locked_credit(creditAll).FormatCtx(ctx))
				// ms.dirty += pi.Amount()
				ms.AddDirty(pi.Amount())
				ms.Log.Infof("bill accepted:%v", pi.Amount().Format100I())
				g.Engine.Exec(ctx, ms.bill.AcceptMax(0))
				alive.Stop()
				if out != nil {
					event := types.Event{Kind: types.EventMoneyCredit, Amount: pi.Amount()}
					// async channel send to avoid deadlock lk.Lock vs <-out
					go func() { out <- event }()
				}
			default:
				ms.Log.Debugf("money.bill poll unknown.")
			}
			return false
		})
	}
	go ms.coin.Run(ctx, alive, func(pi money.PollItem) bool {
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
			g.ClientBegin()
			if pi.DataCashbox {
				if err := ms.coinCashbox.Add(pi.DataNominal, uint(pi.DataCount)); err != nil {
					g.Error(errors.Annotatef(err, "%s cashbox.Add n=%v c=%d", tag, pi.DataNominal, pi.DataCount))
					break
				}
			}
			err := ms.coinCredit.Add(pi.DataNominal, uint(pi.DataCount))
			if err != nil {
				g.Error(errors.Annotatef(err, "%s credit.Add n=%v c=%d", tag, pi.DataNominal, pi.DataCount))
				break
			}
			_ = ms.coin.TubeStatus()
			_ = ms.coin.ExpansionDiagStatus(nil)
			// ms.dirty += pi.Amount()
			ms.AddDirty(pi.Amount())
			ms.Log.Infof("coin accepted:%v", pi.Amount().Format100I())
			alive.Stop()
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

	select {
	case <-alive.WaitChan():
		return nil
	case <-stopAccept:
		alive.Stop()
		alive.Wait()
		return ms.SetAcceptMax(ctx, 0)
	}
}
