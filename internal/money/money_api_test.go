package money

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb/bill"
	state_new "github.com/AlexTransit/vender/internal/state/new"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBiller wraps bill.Stub (which already satisfies bill.Biller) and lets
// tests control EscrowAmount() and observe SendCommand() calls — bill.Stub
// alone always reports EscrowAmount()=0 and swallows commands silently, which
// makes it impossible to exercise the escrow branches in money_api.go.
type mockBiller struct {
	bill.Stub
	escrow   currency.Amount
	commands []bill.BillCommand
}

func (m *mockBiller) EscrowAmount() currency.Amount  { return m.escrow }
func (m *mockBiller) SendCommand(c bill.BillCommand) { m.commands = append(m.commands, c) }

// newTestMoneySystem builds a MoneySystem the same way production does
// (Start()), then pins bill/coin to deterministic test doubles so these
// tests don't depend on hardware or on state leaked from other tests
// (coin.CoinValidator is a package-level var, shared across the test binary).
func newTestMoneySystem(t *testing.T) (context.Context, *MoneySystem, *mockBiller) {
	t.Helper()
	ctx, _ := state_new.NewTestContext(t, "", "")

	ms := &MoneySystem{}
	require.NoError(t, ms.Start(ctx))

	mb := &mockBiller{}
	ms.bill = mb
	ms.CoinValidator = nil

	return ctx, ms, mb
}

// addCredit injects credit directly into the internal nominal groups,
// bypassing SetValid()'s "known nominal" restriction (there is no real
// validator in these tests to declare supported nominals) — mirrors the fix
// applied to XXX_InjectCoin for the same reason.
func addCredit(t *testing.T, ms *MoneySystem, billAmount, coinAmount currency.Amount) {
	t.Helper()
	ms.lk.Lock()
	defer ms.lk.Unlock()
	if billAmount > 0 {
		n := currency.Nominal(billAmount)
		ms.billCredit.EnsureValid(n)
		require.NoError(t, ms.billCredit.Add(n))
	}
	if coinAmount > 0 {
		n := currency.Nominal(coinAmount)
		ms.coinCredit.EnsureValid(n)
		require.NoError(t, ms.coinCredit.Add(n))
	}
}

func TestGetCredit(t *testing.T) {
	_, ms, _ := newTestMoneySystem(t)
	assert.Equal(t, currency.Amount(0), ms.GetCredit())

	addCredit(t, ms, 500, 300)
	assert.Equal(t, currency.Amount(800), ms.GetCredit())
}

func TestBillEscrow(t *testing.T) {
	_, ms, mb := newTestMoneySystem(t)
	assert.Equal(t, currency.Amount(0), ms.BillEscrow())

	mb.escrow = 250
	assert.Equal(t, currency.Amount(250), ms.BillEscrow())
}

func TestBillEscrowToStacker(t *testing.T) {
	t.Run("no escrow, no command sent", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		ms.BillEscrowToStacker()
		assert.Empty(t, mb.commands)
	})

	t.Run("escrow present, accept sent", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		mb.escrow = 100
		ms.BillEscrowToStacker()
		require.Len(t, mb.commands, 1)
		assert.Equal(t, bill.Accept, mb.commands[0])
	})
}

func TestBillEscrowReject(t *testing.T) {
	t.Run("no escrow, no command sent", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		ms.BillEscrowReject()
		assert.Empty(t, mb.commands)
	})

	t.Run("escrow present, reject sent", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		mb.escrow = 100
		ms.BillEscrowReject()
		require.Len(t, mb.commands, 1)
		assert.Equal(t, bill.Reject, mb.commands[0])
	})
}

func TestWaitEscrowAccept(t *testing.T) {
	t.Run("credit already covers amount: no wait, no command", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		addCredit(t, ms, 0, 1000)
		assert.False(t, ms.WaitEscrowAccept(700))
		assert.Empty(t, mb.commands)
	})

	t.Run("insufficient credit, escrowed bill present: escrow accepted", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		mb.escrow = 200
		// billCredit already includes the escrowed bill's value — this is how
		// AcceptCredit's InEscrow handler behaves (it calls billCredit.Add()
		// the moment a bill enters escrow, before it's confirmed/stacked).
		addCredit(t, ms, 200, 100) // bc(200)-ec(200)+cc(100) = 100 < amount(500)
		assert.True(t, ms.WaitEscrowAccept(500))
		require.Len(t, mb.commands, 1)
		assert.Equal(t, bill.Accept, mb.commands[0])
	})

	// WaitEscrowAccept computes bc-ec+cc using currency.Amount, which is
	// uint32 — unsigned. If ec (escrow) ever exceeds bc (bill credit), the
	// subtraction wraps around instead of going negative, and the comparison
	// against amount silently does the wrong thing. In production this
	// shouldn't happen because AcceptCredit adds a bill to billCredit at the
	// same moment it enters escrow (bc should always be >= ec) — but nothing
	// enforces that invariant here, so a future change upstream could
	// reintroduce this quietly. Documenting current (broken) behavior rather
	// than silently "fixing" the arithmetic, since the correct fix depends on
	// intended semantics this test can't decide on its own.
	t.Run("KNOWN ISSUE: ec > bc underflows unsigned Amount, wait wrongly reported false", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		mb.escrow = 200
		addCredit(t, ms, 0, 100) // bc(0) < ec(200): shouldn't happen in practice
		assert.False(t, ms.WaitEscrowAccept(500), "documents the underflow bug — this SHOULD be true")
		assert.Empty(t, mb.commands)
	})

	t.Run("insufficient credit, nothing escrowed: wait but no command", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		addCredit(t, ms, 0, 100)
		assert.True(t, ms.WaitEscrowAccept(500))
		assert.Empty(t, mb.commands) // guarded by EscrowAmount() > 0 in BillEscrowToStacker
	})
}

func TestWithdrawPrepare(t *testing.T) {
	t.Run("success: credit covers amount", func(t *testing.T) {
		ctx, ms, _ := newTestMoneySystem(t)
		addCredit(t, ms, 0, 1000)

		require.NoError(t, ms.WithdrawPrepare(ctx, 700))
		assert.Equal(t, currency.Amount(0), ms.GetCredit(), "credit is consumed on withdraw")
		assert.Equal(t, currency.Amount(700), ms.GetDirty())
	})

	t.Run("insufficient credit: refused, state unchanged", func(t *testing.T) {
		ctx, ms, _ := newTestMoneySystem(t)
		addCredit(t, ms, 0, 300)

		err := ms.WithdrawPrepare(ctx, 700)
		assert.True(t, errors.Is(err, ErrNeedMoreMoney), "expected %v, got %v", ErrNeedMoreMoney, err)
		assert.Equal(t, currency.Amount(300), ms.GetCredit(), "credit must not be touched on refusal")
		assert.Equal(t, currency.Amount(0), ms.GetDirty())
	})
}

func TestWithdrawCommit(t *testing.T) {
	ctx, ms, _ := newTestMoneySystem(t)
	addCredit(t, ms, 0, 500)
	ms.SetDirty(500)

	require.NoError(t, ms.WithdrawCommit(ctx, 500))
	assert.Equal(t, currency.Amount(0), ms.GetCredit())
	assert.Equal(t, currency.Amount(0), ms.GetDirty())
}

func TestGiftCredit(t *testing.T) {
	ctx, ms, _ := newTestMoneySystem(t)
	assert.Equal(t, currency.Amount(0), ms.GetGiftCredit())

	ms.SetGiftCredit(ctx, 500)
	assert.Equal(t, currency.Amount(500), ms.GetGiftCredit())

	ms.SetGiftCredit(ctx, 200)
	assert.Equal(t, currency.Amount(200), ms.GetGiftCredit(), "second call overwrites, doesn't add")
}

func TestReturnDirty(t *testing.T) {
	_, ms, _ := newTestMoneySystem(t)
	ms.SetDirty(300)

	// No CoinValidator configured (as in any test/offline environment) —
	// change cannot physically be dispensed, so the call must fail loudly
	// rather than pretend success.
	err := ms.ReturnDirty()
	assert.True(t, errors.Is(err, ErrCoinAcceptorOffline), "expected %v, got %v", ErrCoinAcceptorOffline, err)
	// dirty must survive a failed return attempt — the amount is still
	// physically stuck in the machine, clearing it here would just make it
	// vanish from the books.
	assert.Equal(t, currency.Amount(300), ms.GetDirty())
}

func TestReturnMoney(t *testing.T) {
	t.Run("nothing to return: succeeds, state stays zero", func(t *testing.T) {
		_, ms, _ := newTestMoneySystem(t)
		require.NoError(t, ms.ReturnMoney())
		assert.Equal(t, currency.Amount(0), ms.GetCredit())
	})

	t.Run("credit present but no coin validator: reports offline", func(t *testing.T) {
		_, ms, _ := newTestMoneySystem(t)
		addCredit(t, ms, 0, 500)

		err := ms.ReturnMoney()
		assert.True(t, errors.Is(err, ErrCoinAcceptorOffline), "expected %v, got %v", ErrCoinAcceptorOffline, err)
		// NOTE: current production behavior clears billCredit/coinCredit/dirty
		// unconditionally before attempting the dispense — so bookkeeping goes
		// to zero here even though the error means the cash was never
		// physically returned. Asserting the current behavior, not endorsing
		// it: see money_api.go ReturnMoney/WithdrawPrepare, both already flag
		// this path as CRITICAL in logs.
		assert.Equal(t, currency.Amount(0), ms.GetCredit())
	})

	t.Run("escrowed bill amount is excluded from cash to return", func(t *testing.T) {
		_, ms, mb := newTestMoneySystem(t)
		mb.escrow = 200
		addCredit(t, ms, 200, 0) // bill credit already counts the escrowed bill
		// cash = billCredit(200) + coinCredit(0) - escrow(200) = 0 -> no dispense attempted
		require.NoError(t, ms.ReturnMoney())
	})
}
