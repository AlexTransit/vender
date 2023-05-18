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
	// BillRun(*alive.Alive, func(money.BillEvent))
	// BillRun(chan<- money.BillEvent, *alive.Alive)

	BillReset() error
	BillStacked() bool
	GetState() BllStateType
}

var _ Biller = &BillValidator{}
var _ Biller = Stub{}

// func (bv *BillValidator) EscrowAccept() engine.Doer { return bv.DoEscrowAccept }
// func (bv *BillValidator) EscrowReject() engine.Doer { return bv.DoEscrowReject }

// func (bv *BillValidator) GetBillEventChannel()  { return chan money.BillEvent }

type Stub struct{}

// func (Stub) AcceptMax(currency.Amount) engine.Doer {
// 	// return engine.Fail{E: errors.NotSupportedf("bill.Stub.AcceptMax")}
// 	return engine.Nothing{}
// }

// func (Stub) Run(ctx context.Context, alive *alive.Alive, fun func(money.PollItem) bool) {
// 	// fun(money.PollItem{
// 	// 	Status: money.StatusFatal,
// 	// 	Error:  errors.NotSupportedf("bill.Stub.Run"),
// 	// })
// 	if alive != nil {
// 		alive.Done()
// 	}
// }

func (Stub) SupportedNominals() []currency.Nominal { return nil }

func (Stub) EscrowAmount() currency.Amount   { return 0 }
func (Stub) EscrowNominal() currency.Nominal { return 0 }

func (Stub) SendCommand(BillCommand) {}

// func (Stub) EscrowAccept() engine.Doer { return engine.Nothing{} }
// func (Stub) EscrowReject() engine.Doer { return engine.Nothing{} }

func (Stub) BillRun(*alive.Alive, func(money.ValidatorEvent)) {}

// func (Stub) BillRun(*alive.Alive, func(money.BillEvent)) {}
// func (Stub) BillRun(chan<- money.BillEvent, *alive.Alive) {}

func (Stub) BillReset() error { return nil }

func (Stub) BillStacked() bool { return false }

func (Stub) GetState() BllStateType { return 0 }
