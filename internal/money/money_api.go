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
	oerr "github.com/juju/errors"
)

var (
	ErrNeedMoreMoney        = errors.New("add-money")
	ErrChangeRetainOverflow = errors.New("ReturnChange(retain>total)")
	ErrCoinAcceptorOffline  = errors.New("coin-acceptor-offline")
)

func (ms *MoneySystem) TestingDispense() {
	if ms.CoinValidator == nil {
		ms.Log.Error(ErrCoinAcceptorOffline)
		return
	}
	ms.CoinValidator.TestingDispense()
}

func (ms *MoneySystem) WaitEscrowAccept(amount currency.Amount) (wait bool) {
	ms.lk.RLock()
	bc := ms.billCredit.Total()
	cc := ms.coinCredit.Total()
	ec := ms.bill.EscrowAmount()
	ms.lk.RUnlock()
	if amount > bc-ec+cc {
		ms.Log.Infof("bill credit:%v coin credit:%v, escrow bill:%v. send command accept escrow", bc.Format100I(), cc.Format100I(), ec.Format100I())
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

func (ms *MoneySystem) BillEscrowReject() {
	if ms.bill.EscrowAmount() > 0 {
		ms.bill.SendCommand(bill.Reject)
	}
}

func (ms *MoneySystem) BillEscrow() currency.Amount {
	return ms.bill.EscrowAmount()
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
	ms.lk.Lock()
	available := ms.billCredit.Total() + ms.coinCredit.Total()
	if available < amount {
		ms.lk.Unlock()
		return ErrNeedMoreMoney
	}
	change := available - amount
	// ms.Log.Debugf("%s. return short change=%s", tag, change.FormatCtx(ctx))
	ms.billCredit.Clear()
	ms.coinCredit.Clear()
	ms.lk.Unlock()
	go func() {
		if ms.CoinValidator == nil {
			ms.Log.WarningF("%s CRITICAL change err=%v", tag, ErrCoinAcceptorOffline)
			ms.SetDirty(amount)
			return
		}
		if err := ms.CoinValidator.Dispense(change); err != nil {
			err = oerr.Annotate(err, tag)
			ms.Log.Errorf("%s CRITICAL change err=%v", tag, err)
		}
		ms.SetDirty(amount)
	}()
	return nil
}

// ----------------------------------------------------------------------

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

func (ms *MoneySystem) ReturnDirty() error {
	ms.lk.Lock()
	dirty := ms.dirty
	ms.lk.Unlock()
	if ms.CoinValidator == nil {
		return ErrCoinAcceptorOffline
	}
	return ms.CoinValidator.ReturnMoney(dirty)
}

func (ms *MoneySystem) ReturnMoney() error {
	ms.lk.Lock()
	cash := ms.billCredit.Total() + ms.coinCredit.Total() - ms.bill.EscrowAmount()
	ms.setDirtyLocked(0)
	ms.billCredit.Clear()
	ms.coinCredit.Clear()
	ms.giftCredit = 0
	ms.lk.Unlock()
	if cash > 0 {
		if ms.CoinValidator == nil {
			return ErrCoinAcceptorOffline
		}
		ms.Log.Infof("return money (%v)", cash)
		return ms.CoinValidator.Dispense(cash)
	}
	return nil
}

func (ms *MoneySystem) locked_zero() {
	// ms.dirty = 0
	ms.setDirtyLocked(0)
	ms.billCredit.Clear()
	ms.coinCredit.Clear()
	ms.giftCredit = 0
}
