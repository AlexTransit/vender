package coin

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
)

// High-level dispense wrapper. Handles:
// - built-in payout or dispense-by-coin using expend strategy
// - give smallest amount >= requested
func (ca *CoinAcceptor) NewGive(requestAmount currency.Amount, over bool, success *currency.NominalGroup) engine.Doer {
	const tag = "coin.give"

	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		var err error
		var successAmount currency.Amount
		g := state.GetGlobal(ctx)
		ca.Device.Log.Debugf("%s requested=%s", tag, requestAmount.FormatCtx(ctx))
		leftAmount := requestAmount // save original requested amount for logs
		success.SetValid(ca.nominals[:])
		if leftAmount == 0 {
			return nil
		}

		// === Try smart dispense-by-coin
		if ca.giveSmart {
			err = ca.giveSmartManual(ctx, leftAmount, success)
			if err != nil {
				return errors.Annotate(err, tag)
			}
			successAmount = success.Total()
			if successAmount == requestAmount {
				return nil
			} else if successAmount > requestAmount {
				panic("code error")
			}
			leftAmount = requestAmount - successAmount
			ca.Device.Log.Errorf("%s fallback to PAYOUT left=%s", tag, leftAmount.FormatCtx(ctx))
		}

		// === Fallback to PAYOUT
		err = g.Engine.Exec(ctx, ca.NewPayout(leftAmount, success))
		if err != nil {
			return errors.Annotate(err, tag)
		}
		successAmount = success.Total()
		if successAmount == requestAmount {
			return nil
		} else if successAmount > requestAmount {
			panic("code error")
		}
		leftAmount = requestAmount - successAmount

		// === Not enough coins for exact payout
		if !over {
			ca.Device.Log.Errorf("%s not enough coins for exact payout and over-compensate disabled in request", tag)
			return currency.ErrNominalCount
		}
		// === Try to give a bit more
		// TODO telemetry
		successAmount = success.Total()
		ca.Device.Log.Errorf("%s dispensed=%s < requested=%s debt=%s",
			tag, successAmount.FormatCtx(ctx), requestAmount.FormatCtx(ctx), leftAmount.FormatCtx(ctx))
		if leftAmount <= currency.Amount(g.Config.Money.ChangeOverCompensate) {
			return g.Engine.Exec(ctx, ca.NewGiveLeastOver(leftAmount, success))
		}
		return currency.ErrNominalCount
	}}
}

func (ca *CoinAcceptor) NewGiveLeastOver(requestAmount currency.Amount, success *currency.NominalGroup) engine.Doer {
	const tag = "coin.give-least-over"

	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		var err error
		g := state.GetGlobal(ctx)
		leftAmount := requestAmount

		nominals := ca.SupportedNominals()
		sort.Slice(nominals, func(i, j int) bool { return nominals[i] < nominals[j] })

		for _, nominal := range nominals {
			namt := currency.Amount(nominal)

			// Round up `leftAmount` to nearest multiple of `nominal`
			payoutAmount := leftAmount + namt - 1 - (leftAmount-1)%namt

			ca.Device.Log.Debugf("%s request=%s left=%s trying nominal=%s payout=%s",
				tag, requestAmount.FormatCtx(ctx), leftAmount.FormatCtx(ctx), namt.FormatCtx(ctx), payoutAmount.FormatCtx(ctx))
			payed := success.Copy()
			payed.Clear()
			// TODO use DISPENSE
			err = g.Engine.Exec(ctx, ca.NewPayout(payoutAmount, payed))
			success.AddFrom(payed)
			payedAmount := payed.Total()
			// ca.Device.Log.Debugf("%s dispense err=%v", tag, err)
			if err != nil {
				return errors.Annotate(err, tag)
			}
			if leftAmount <= payedAmount {
				return nil
			}
			leftAmount -= payedAmount
		}
		return errors.Annotate(currency.ErrNominalCount, tag)
	}}
}

func (ca *CoinAcceptor) giveSmartManual(ctx context.Context, amount currency.Amount, success *currency.NominalGroup) error {
	const tag = "coin.give-smart/manual"
	var err error

	if err = ca.TubeStatus(); err != nil {
		return err
	}
	tubeCoins := ca.Tubes()
	if tubeCoins.Total() < amount {
		ca.Device.Log.Errorf("%s not enough coins in tubes for amount=%s", tag, amount.FormatCtx(ctx))
		return nil // TODO more sensible error
	}

	config := state.GetGlobal(ctx).Config
	_ = config
	// TODO read preferred strategy from config
	strategy := currency.NewExpendLeastCount()
	if !strategy.Validate() {
		ca.Device.Log.Errorf("%s config strategy=%v is not available, using fallback", tag, strategy)
		strategy = currency.NewExpendLeastCount()
		if !strategy.Validate() {
			panic("code error fallback coin strategy validate")
		}
	}

	ng := new(currency.NominalGroup)
	ng.SetValid(ca.nominals[:])
	if err = tubeCoins.Withdraw(ng, amount, strategy); err != nil {
		// TODO telemetry
		ca.Device.Log.Errorf("%s failed to calculate NominalGroup for dispense mode", tag)
		return nil
	}

	err = ca.dispenseGroup(ctx, ng, success)
	ca.Device.Log.Debugf("%s success=%s", tag, success.String())
	return errors.Annotate(err, tag)
}

func (ca *CoinAcceptor) dispenseGroup(ctx context.Context, request, success *currency.NominalGroup) error {
	const tag = "coin.dispense-group"
	g := state.GetGlobal(ctx)

	return request.Iter(func(nominal currency.Nominal, count uint) error {
		ca.Device.Log.Debugf("%s n=%s c=%d", tag, currency.Amount(nominal).FormatCtx(ctx), count)
		if count == 0 {
			return nil
		}
		d := ca.NewDispense(nominal, uint8(count))
		if err := g.Engine.Exec(ctx, d); err != nil {
			err = errors.Annotatef(err, "%s nominal=%s count=%d", tag, currency.Amount(nominal).FormatCtx(ctx), count)
			ca.Device.Log.Error(err)
			return errors.Annotate(err, tag)
		}
		return success.AddMany(nominal, count)
	})
}

// MDB command DISPENSE (0d)
func (ca *CoinAcceptor) NewDispense(nominal currency.Nominal, count uint8) engine.Doer {
	const tag = "coin.dispense"

	command := func(ctx context.Context) error {
		if count > 15 { // count must fit into 4 bits
			panic(fmt.Sprintf("code error %s count=%d > 15", tag, count))
		}
		coinType := ca.nominalCoinType(nominal)
		if coinType < 0 {
			return errors.Errorf("%s not supported for nominal=%s", tag, currency.Amount(nominal).FormatCtx(ctx))
		}

		request := mdb.MustPacketFromBytes([]byte{0x0d, (count << 4) + uint8(coinType)}, true)
		err := ca.Device.TxMaybe(request, nil) // TODO check known/other
		return errors.Annotate(err, tag)
	}
	pollFun := func(p mdb.Packet) (bool, error) {
		bs := p.Bytes()
		if len(bs) == 0 {
			return true, nil
		}
		pi, _ := ca.parsePollItem(bs[0], 0)
		// ca.Device.Log.Debugf("%s poll=%x parsed=%v", tag, bs, pi)
		switch pi.Status {
		case money.StatusBusy:
			return false, nil
		case money.StatusError, money.StatusFatal: // tube jam, etc
			return true, pi.Error
		}
		return true, errors.Errorf("unexpected response=%x", bs)
	}

	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		var err error
		g := state.GetGlobal(ctx)
		// TODO  avoid double mutex acquire
		if err = ca.TubeStatus(); err != nil {
			return errors.Annotate(err, tag)
		}
		tubesBefore := ca.Tubes()
		var countBefore uint
		if countBefore, err = tubesBefore.Get(nominal); err != nil {
			return errors.Annotate(err, tag)
		} else if countBefore < uint(count) {
			err = currency.ErrNominalCount
			return errors.Annotate(err, tag)
		}

		ca.pollmu.Lock()
		defer ca.pollmu.Unlock()

		if err = command(ctx); err != nil {
			return errors.Annotate(err, tag)
		}
		totalTimeout := ca.dispenseTimeout * time.Duration(count)
		doPoll := ca.Device.NewPollLoop(tag, ca.Device.PacketPoll, totalTimeout, pollFun)
		if err = g.Engine.Exec(ctx, doPoll); err != nil {
			return errors.Annotate(err, tag)
		}

		if err = ca.TubeStatus(); err != nil {
			return errors.Annotate(err, tag)
		}
		_ = ca.ExpansionDiagStatus(nil)
		tubesAfter := ca.Tubes()
		var countAfter uint
		if countAfter, err = tubesAfter.Get(nominal); err != nil {
			return errors.Annotate(err, tag)
		}

		diff := int(countBefore) - int(countAfter)
		if diff != int(count) {
			return errors.Errorf("%s nominal=%s requested=%d diff=%d", tag, currency.Amount(nominal).FormatCtx(ctx), count, diff)
		}
		return nil
	}}
}

// MDB command PAYOUT (0f02)
func (ca *CoinAcceptor) NewPayout(amount currency.Amount, success *currency.NominalGroup) engine.Doer {
	const tag = "coin.payout"
	ca.Device.Log.Debugf("%s sf=%v amount=%s", tag, ca.scalingFactor, amount.Format100I())
	arg := amount / currency.Amount(ca.scalingFactor)

	doPayout := engine.Func{Name: tag + "/command", F: func(ctx context.Context) error {
		request := mdb.MustPacketFromBytes([]byte{0x0f, 0x02, byte(arg)}, true)
		err := ca.Device.TxMaybe(request, nil)
		return errors.Annotate(err, tag)
	}}
	doStatus := engine.Func{Name: tag + "/status", F: func(ctx context.Context) error {
		response := mdb.Packet{}
		err := ca.Device.TxMaybe(packetPayoutStatus, &response)
		if err != nil {
			return errors.Annotate(err, tag)
		}
		for i, count := range response.Bytes() {
			if count > 0 {
				nominal := ca.nominals[i]
				if err := success.AddMany(nominal, uint(count)); err != nil {
					return errors.Annotate(err, tag)
				}
			}
		}
		return nil
	}}
	pollFun := func(p mdb.Packet) (bool, error) {
		bs := p.Bytes()
		if len(bs) == 0 {
			return true, nil
		} else if len(bs) == 1 && bs[0] == 0x02 { // Changer Payout Busy
			return false, nil
		}
		b1 := bs[0]
		b2 := byte(0)
		if len(bs) >= 2 {
			b2 = bs[1]
		}
		pi, _ := ca.parsePollItem(b1, b2)
		ca.Device.Log.Errorf("PLEASE REPORT PAYOUT POLL response=%x pi=%v", bs, pi)
		return false, nil
	}

	return engine.NewSeq(tag).
		Append(doPayout).
		Append(engine.Sleep{Duration: ca.Device.DelayNext}).
		Append(ca.Device.NewPollLoop(tag, ca.Device.PacketPoll, ca.dispenseTimeout*4, pollFun)).
		Append(doStatus)
}
