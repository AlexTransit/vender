package coin

import (
	"context"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

const deviceName = "coin"

func Enum(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &CoinAcceptor{}
	// TODO dev.init() without IO
	// TODO g.RegisterDevice(deviceName, dev, dev.Probe)
	return g.RegisterDevice(deviceName, dev, func() error { return dev.init(ctx) })
}

type Coiner interface {
	AcceptMax(currency.Amount) engine.Doer
	ExpansionDiagStatus(*DiagResult) error
	SupportedNominals() []currency.Nominal
	NewGive(currency.Amount, bool, *currency.NominalGroup) engine.Doer
	TubeStatus() error
	Tubes() *currency.NominalGroup
	TestingDispense()
	CoinRun(*alive.Alive, func(money.ValidatorEvent))
	DisableAccept()
	Dispence(currency.Amount) error
}

var _ Coiner = &CoinAcceptor{}
var _ Coiner = Stub{}

type Stub struct{}

func (Stub) CoinRun(*alive.Alive, func(money.ValidatorEvent)) {}

func (Stub) DisableAccept()   {}
func (Stub) TestingDispense() {}

func (Stub) Dispence(currency.Amount) error { return nil }

func (Stub) AcceptMax(currency.Amount) engine.Doer {
	return engine.Fail{E: errors.NotSupportedf("coin.Stub.AcceptMax")}
}

func (Stub) ExpansionDiagStatus(*DiagResult) error {
	return errors.NotSupportedf("coin.Stub.ExpansionDiagStatus")
}

func (Stub) SupportedNominals() []currency.Nominal { return nil }

func (Stub) NewGive(currency.Amount, bool, *currency.NominalGroup) engine.Doer {
	return engine.Nothing{}
}

func (Stub) TubeStatus() error { return nil }

func (Stub) Tubes() *currency.NominalGroup {
	ng := &currency.NominalGroup{}
	ng.SetValid(nil)
	return ng
}
