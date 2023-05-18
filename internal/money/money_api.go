// Package money provides high-level interaction with money devices.
// Overview:
//   - head->money: enable accepting coins and bills
//     inits required devices, starts polling
//   - (parsed device status)
//     money->ui: X money inserted
//   - head->money: (ready to serve product) secure transaction, release change
package money

import (
	"context"
	"errors"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb/bill"
	"github.com/AlexTransit/vender/internal/state"
	oerr "github.com/juju/errors"
)

var (
	ErrNeedMoreMoney        = errors.New("add-money")
	ErrChangeRetainOverflow = errors.New("ReturnChange(retain>total)")
)

func (ms *MoneySystem) WaitEscrowAccept(amount currency.Amount) (wait bool) {
	bc := ms.billCredit.Total()
	cc := ms.coinCredit.Total()
	ec := ms.bill.EscrowAmount()
	if amount > bc-ec+cc {
		ms.Log.Infof("bill credit:%v coin credit:%v, escrow bill:%v. send command accept escrow", bc, cc, ec)
		ms.BillEscrowToStacker()
		return true
	}
	return false
}

func (ms *MoneySystem) BillEscrowToStacker() {
	if ms.bill.EscrowAmount() > 0 {
		ms.bill.SendCommand(bill.Accept)
	}
}

func (ms *MoneySystem) GetCredit() currency.Amount {
	ms.lk.RLock()
	defer ms.lk.RUnlock()
	return ms.billCredit.Total() + ms.coinCredit.Total()
}

// возвращаем сдачу. если неполучиться приготовить то вернем стоимость напитка
func (ms *MoneySystem) WithdrawPrepare(ctx context.Context, amount currency.Amount) error {
	const tag = "money.withdraw-prepare"
	ms.Log.Debugf("%s amount=%s", tag, amount.FormatCtx(ctx))
	available := ms.GetCredit()
	if available < amount {
		return ErrNeedMoreMoney
	}
	change := available - amount
	ms.Log.Debugf("%s. return short change=%s", tag, change.FormatCtx(ctx))
	go func() {
		if err := ms.locked_payout(ctx, change); err != nil {
			err = oerr.Annotate(err, tag)
			ms.Log.Errorf("%s CRITICAL change err=%v", tag, err)
			state.GetGlobal(ctx).Tele.Error(err)
		}
		ms.SetDirty(amount)
	}()
	return nil

	// old code
	// go func() {
	// 	ms.lk.Lock()
	// 	defer ms.lk.Unlock()
	// 	if err := ms.locked_payout(ctx, change); err != nil {
	// 		err = oerr.Annotate(err, tag)
	// 		ms.Log.Errorf("%s CRITICAL change err=%v", tag, err)
	// 		state.GetGlobal(ctx).Tele.Error(err)
	// 	}
	// 	// billEscrowAmount := ms.bill.EscrowAmount()
	// 	// if billEscrowAmount != 0 {
	// 	// 	if err := g.Engine.Exec(ctx, ms.bill.EscrowAccept()); err != nil {
	// 	// 		err = errors.Annotate(err, tag+"CRITICAL EscrowAccept")
	// 	// 		ms.Log.Error(err)
	// 	// 	} else {
	// 	// 		// ms.dirty += billEscrowAmount
	// 	// 		ms.AddDirty(billEscrowAmount)
	// 	// 	}
	// 	// }
	// 	// if ms.dirty != amount {
	// 	if ms.GetDirty() != amount {
	// 		ms.Log.Errorf("%s (WithdrawPrepare) CRITICAL amount=%s dirty=%s", tag, amount.FormatCtx(ctx), ms.dirty.FormatCtx(ctx))
	// 	}
	// }()

}

// ----------------------------------------------------------------------

// type creditFlag uint16

// const (
// 	creditInvalid = creditFlag(0)
// 	creditCash    = creditFlag(1 << iota)
// 	creditEscrow
// 	creditGift
// 	creditAll = creditCash | creditEscrow | creditGift
// )

// func (cf creditFlag) Contains(sub creditFlag) bool { return cf&sub != 0 }

// func (ms *MoneySystem) locked_credit(flag creditFlag) currency.Amount {
// 	result := currency.Amount(0)
// 	if flag.Contains(creditEscrow) {
// 		result += ms.bill.EscrowAmount()
// 	}
// 	if flag.Contains(creditCash) {
// 		// result += ms.dirty
// 		result += ms.GetDirty()
// 		// result += ms.billCredit.Total()
// 		// result += ms.coinCredit.Total()
// 	}
// 	if flag.Contains(creditGift) {
// 		result += ms.giftCredit
// 	}
// 	return result
// }

// func (ms *MoneySystem) Credit(ctx context.Context) currency.Amount {
// 	ms.lk.RLock()
// 	defer ms.lk.RUnlock()
// 	return ms.billCredit.Total() + ms.coinCredit.Total()
// }

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

// WithdrawCommit Store spending to durable memory, no user initiated return after this point.
func (ms *MoneySystem) WithdrawCommit(ctx context.Context, amount currency.Amount) error {
	const tag = "money.withdraw-commit"
	ms.lk.Lock()
	defer ms.lk.Unlock()
	ms.Log.Debugf("%s amount=%s dirty=%s", tag, amount.FormatCtx(ctx), ms.dirty.FormatCtx(ctx))
	ms.locked_zero()
	return nil
}

func (ms *MoneySystem) ReturnMoney(ctx context.Context) error {
	const tag = "money-abort"
	cash := ms.billCredit.Total() + ms.coinCredit.Total() - ms.bill.EscrowAmount()
	// escrow bill return before stop bill
	if err := ms.locked_payout(ctx, cash); err != nil {
		err = oerr.Annotate(err, tag)
		ms.Log.Errorf("%s CRITICAL change err=%v", tag, err)
		state.GetGlobal(ctx).Tele.Error(err)
	}
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
	// const tag = "money.payout"
	var err error
	g := state.GetGlobal(ctx)
	// TODO bill.recycler-release
	// coin change
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
		err = oerr.Annotatef(err, "debt=%s", debt.FormatCtx(ctx))
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
