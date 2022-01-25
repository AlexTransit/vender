// Package money provides high-level interaction with money devices.
// Overview:
// - head->money: enable accepting coins and bills
//   inits required devices, starts polling
// - (parsed device status)
//   money->ui: X money inserted
// - head->money: (ready to serve product) secure transaction, release change
package money

import (
	"context"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
)

var (
	ErrNeedMoreMoney        = errors.New("add-money")
	ErrChangeRetainOverflow = errors.New("ReturnChange(retain>total)")
)

type creditFlag uint16

const (
	creditInvalid = creditFlag(0)
	creditCash    = creditFlag(1 << iota)
	creditEscrow
	creditGift
	creditAll = creditCash | creditEscrow | creditGift
)

func (cf creditFlag) Contains(sub creditFlag) bool { return cf&sub != 0 }

func (ms *MoneySystem) locked_credit(flag creditFlag) currency.Amount {
	result := currency.Amount(0)
	// if flag.Contains(creditEscrow) {
	// 	result += ms.bill.EscrowAmount()
	// }
	if flag.Contains(creditCash) {
		// result += ms.dirty
		result += ms.GetDirty()
		// result += ms.billCredit.Total()
		// result += ms.coinCredit.Total()
	}
	if flag.Contains(creditGift) {
		result += ms.giftCredit
	}
	return result
}

func (ms *MoneySystem) Credit(ctx context.Context) currency.Amount {
	ms.lk.RLock()
	defer ms.lk.RUnlock()
	return ms.locked_credit(creditAll)
}

// GetGiftCredit TODO replace with WithdrawPrepare() -> []Spending{Cash: ..., Gift: ...}
func (ms *MoneySystem) GetGiftCredit() currency.Amount {
	ms.lk.RLock()
	c := ms.giftCredit
	ms.lk.RUnlock()
	return c
}

func (ms *MoneySystem) SetGiftCredit(ctx context.Context, value currency.Amount) {
	const tag = "money.set-gift-credit"

	ms.lk.Lock()
	// copy both values to release lock ASAP
	before, after := ms.giftCredit, value
	ms.giftCredit = after
	ms.lk.Unlock()
	ms.Log.Infof("%s before=%s after=%s", tag, before.FormatCtx(ctx), after.FormatCtx(ctx))

	// TODO notify ui-front
}

func (ms *MoneySystem) WithdrawPrepare(ctx context.Context, amount currency.Amount) error {
	const tag = "money.withdraw-prepare"
	// g := state.GetGlobal(ctx)

	ms.lk.Lock()
	defer ms.lk.Unlock()

	ms.Log.Debugf("%s amount=%s", tag, amount.FormatCtx(ctx))
	available := ms.locked_credit(creditAll)
	if available < amount {
		return ErrNeedMoreMoney
	}

	change := currency.Amount(0)
	// Don't give change from gift money.
	if cash := ms.locked_credit(creditCash | creditEscrow); cash > amount {
		change = cash - amount
	}

	go func() {
		ms.lk.Lock()
		defer ms.lk.Unlock()

		if err := ms.locked_payout(ctx, change); err != nil {
			err = errors.Annotate(err, tag)
			ms.Log.Errorf("%s CRITICAL change err=%v", tag, err)
			state.GetGlobal(ctx).Tele.Error(err)
		}

		// billEscrowAmount := ms.bill.EscrowAmount()
		// if billEscrowAmount != 0 {
		// 	if err := g.Engine.Exec(ctx, ms.bill.EscrowAccept()); err != nil {
		// 		err = errors.Annotate(err, tag+"CRITICAL EscrowAccept")
		// 		ms.Log.Error(err)
		// 	} else {
		// 		// ms.dirty += billEscrowAmount
		// 		ms.AddDirty(billEscrowAmount)
		// 	}
		// }

		// if ms.dirty != amount {
		if ms.GetDirty() != amount {
			ms.Log.Errorf("%s (WithdrawPrepare) CRITICAL amount=%s dirty=%s", tag, amount.FormatCtx(ctx), ms.dirty.FormatCtx(ctx))
		}
	}()

	return nil
}

// WithdrawCommit Store spending to durable memory, no user initiated return after this point.
func (ms *MoneySystem) WithdrawCommit(ctx context.Context, amount currency.Amount) error {
	const tag = "money.withdraw-commit"

	ms.lk.Lock()
	defer ms.lk.Unlock()

	ms.Log.Debugf("%s amount=%s dirty=%s", tag, amount.FormatCtx(ctx), ms.dirty.FormatCtx(ctx))
	// if ms.dirty != amount {
	// if ms.GetDirty() != amount {
	// 	ms.Log.Errorf("%s (WithdrawCommit)CRITICAL amount=%s dirty=%s", tag, amount.FormatCtx(ctx), ms.dirty.FormatCtx(ctx))
	// }
	ms.locked_zero()
	return nil
}

// Abort Release bill escrow + inserted coins
// returns error *only* if unable to return all money
func (ms *MoneySystem) Abort(ctx context.Context) error {
	const tag = "money-abort"
	ms.lk.Lock()
	defer ms.lk.Unlock()

	cash := ms.locked_credit(creditCash | creditEscrow)
	ms.Log.Debugf("%s cash=%s", tag, cash.FormatCtx(ctx))

	if err := ms.locked_payout(ctx, cash); err != nil {
		err = errors.Annotate(err, tag)
		state.GetGlobal(ctx).Tele.Error(err)
		return err
	}

	// if ms.dirty != 0 {
	if ms.GetDirty() != 0 {
		ms.Log.Errorf("%s CRITICAL (debt or code error) dirty=%s", tag, ms.dirty.FormatCtx(ctx))
	}
	// ms.dirty = 0
	ms.SetDirty(0)
	ms.billCredit.Clear()
	ms.coinCredit.Clear()
	ms.giftCredit = 0

	return nil
}

func (ms *MoneySystem) locked_payout(ctx context.Context, amount currency.Amount) error {
	if amount == 0 {
		return nil
	}

	const tag = "money.payout"
	var err error
	g := state.GetGlobal(ctx)

	billEscrowAmount := ms.bill.EscrowAmount()
	if billEscrowAmount != 0 && billEscrowAmount <= amount {
		if err = g.Engine.Exec(ctx, ms.bill.EscrowReject()); err != nil {
			return errors.Annotate(err, tag)
		}
		amount -= billEscrowAmount
		if amount == 0 {
			return nil
		}
	}

	// TODO bill.recycler-release

	tubeBefore := ms.coin.Tubes()
	ms.Log.Infof("tubes before dispense (%v)", tubeBefore)
	dispensed := new(currency.NominalGroup)
	err = g.Engine.Exec(ctx, ms.coin.NewGive(amount, true, dispensed))
	// Warning: `dispensedAmount` may be more or less than `amount`
	dispensedAmount := dispensed.Total()
	// ms.Log.Debugf("%s coin total dispensed=%s", tag, dispensedAmount.FormatCtx(ctx))
	ms.Log.Infof("coin total dispensed=%s", dispensedAmount.FormatCtx(ctx))
	ms.coin.TubeStatus()
	tubeAfter := ms.coin.Tubes()
	ms.Log.Infof("tubes after dispense  (%v)", tubeAfter)
	// AlexM add check  if (tubeBefore - dispensed != tubeAfter) send tele error
	// ms.g.Error(errors.Errorf("money timeout lost (%v)", credit))
	if dispensedAmount < amount {
		debt := amount - dispensedAmount
		err = errors.Annotatef(err, "debt=%s", debt.FormatCtx(ctx))
	}
	if dispensedAmount <= amount {
		// ms.dirty -= dispensedAmount
		ms.AddDirty(-dispensedAmount)
	} else {
		// ms.dirty -= amount
		ms.AddDirty(-amount)
	}
	return err
}

func (ms *MoneySystem) locked_zero() {
	// ms.dirty = 0
	ms.SetDirty(0)
	ms.billCredit.Clear()
	ms.coinCredit.Clear()
	ms.giftCredit = 0
}
