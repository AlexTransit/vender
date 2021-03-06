package bill

import (
	"context"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/internal/engine"
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
	AcceptMax(currency.Amount) engine.Doer
	Run(context.Context, *alive.Alive, func(money.PollItem) bool)
	SupportedNominals() []currency.Nominal
	EscrowAmount() currency.Amount
	EscrowAccept() engine.Doer
	EscrowReject() engine.Doer
}

var _ Biller = &BillValidator{}
var _ Biller = Stub{}

func (bv *BillValidator) EscrowAccept() engine.Doer { return bv.DoEscrowAccept }
func (bv *BillValidator) EscrowReject() engine.Doer { return bv.DoEscrowReject }

type Stub struct{}

func (Stub) AcceptMax(currency.Amount) engine.Doer {
	// return engine.Fail{E: errors.NotSupportedf("bill.Stub.AcceptMax")}
	return engine.Nothing{}
}

func (Stub) Run(ctx context.Context, alive *alive.Alive, fun func(money.PollItem) bool) {
	// fun(money.PollItem{
	// 	Status: money.StatusFatal,
	// 	Error:  errors.NotSupportedf("bill.Stub.Run"),
	// })
	if alive != nil {
		alive.Done()
	}
}

func (Stub) SupportedNominals() []currency.Nominal { return nil }

func (Stub) EscrowAmount() currency.Amount { return 0 }

// func (Stub) EscrowAccept() engine.Doer { return engine.Fail{E: errors.NotSupportedf("bill.Stub.EscrowAccept")} }
// func (Stub) EscrowReject() engine.Doer { return engine.Fail{E: errors.NotSupportedf("bill.Stub.EscrowReject")} }
func (Stub) EscrowAccept() engine.Doer { return engine.Nothing{} }
func (Stub) EscrowReject() engine.Doer { return engine.Nothing{} }
