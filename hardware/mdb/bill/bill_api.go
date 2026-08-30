package bill

import (
	"context"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/temoto/alive/v2"
)

const deviceName = "bill"

func Enum(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &BillValidator{}
	// TODO dev.init() without IO
	// TODO g.RegisterDevice(deviceName, dev, dev.Probe)
	return g.RegisterDevice(deviceName, dev, func() error { return dev.init(ctx) })
}

type Biller interface {
	// AcceptMax(currency.Amount) engine.Doer

	SupportedNominals() []currency.Nominal
	EscrowAmount() currency.Amount
	EscrowNominal() currency.Nominal
	// EscrowAccept() engine.Doer
	// EscrowReject() engine.Doer

	SendCommand(BillCommand)
	BillRun(*alive.Alive, func(money.ValidatorEvent))
	BillReset() error
	BillStacked() bool
	GetState() BllStateType
	DisableAccept()
}

var (
	_ Biller = &BillValidator{}
	_ Biller = Stub{}
)

type Stub struct{}

func (Stub) SupportedNominals() []currency.Nominal { return nil }

func (Stub) EscrowAmount() currency.Amount { return 0 }

func (Stub) EscrowNominal() currency.Nominal { return 0 }

func (Stub) SendCommand(BillCommand) {}

// BillRun implements Biller. There is no real device behind the stub, so it
// signals completion immediately — mirroring how AcceptCredit handles a nil
// CoinValidator. Without this, callers that alive.Add() a slot per validator
// and alive.Wait() for it (e.g. ui.onFrontSelect) deadlock forever whenever
// no bill acceptor is configured.
func (Stub) BillRun(a *alive.Alive, _ func(money.ValidatorEvent)) { a.Done() }

func (Stub) BillReset() error { return nil }

func (Stub) BillStacked() bool { return false }

func (Stub) GetState() BllStateType { return 0 }

func (Stub) DisableAccept() {}
