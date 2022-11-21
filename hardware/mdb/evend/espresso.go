package evend

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
)

const DefaultEspressoTimeout = 30 * time.Second

type DeviceEspresso struct {
	Generic

	timeout time.Duration
}

func (devEspr *DeviceEspresso) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	espressoConfig := &g.Config.Hardware.Evend.Espresso
	devEspr.timeout = helpers.IntSecondDefault(espressoConfig.TimeoutSec, DefaultEspressoTimeout)
	devEspr.Generic.Init(ctx, 0xe8, "espresso", proto2)

	g.Engine.Register(devEspr.name+".grind", devEspr.NewGrind())
	g.Engine.Register(devEspr.name+".grindNoWait", devEspr.GrindNoWait())
	g.Engine.Register(devEspr.name+".waitDone", devEspr.WaitDone())
	g.Engine.Register(devEspr.name+".press", devEspr.NewPress())
	g.Engine.Register(devEspr.name+".dispose", devEspr.NewRelease())
	g.Engine.Register(devEspr.name+".heat_on", devEspr.NewHeat(true))
	g.Engine.Register(devEspr.name+".heat_off", devEspr.NewHeat(false))

	err := devEspr.Generic.FIXME_initIO(ctx)
	return errors.Annotate(err, devEspr.name+".init")
}

func (devEspr *DeviceEspresso) NewGrind() engine.Doer {
	tag := devEspr.name + ".grind"
	return engine.NewSeq(tag).
		Append(devEspr.Generic.NewWaitReady(tag)).
		Append(devEspr.Generic.NewAction(tag, 0x01)).
		// TODO expect delay like in cup dispense, ignore immediate error, retry
		Append(devEspr.Generic.NewWaitDone(tag, devEspr.timeout))
}

func (devEspr *DeviceEspresso) GrindNoWait() engine.Doer {
	tag := devEspr.name + ".grindNoWait"
	return engine.NewSeq(tag).
		Append(devEspr.Generic.NewAction(tag, 0x01))
}

func (devEspr *DeviceEspresso) WaitDone() engine.Doer {
	tag := devEspr.name + ".waitDone"
	return engine.NewSeq(tag).
		Append(devEspr.Generic.NewWaitReady(tag)).
		Append(devEspr.Generic.NewWaitDone(tag, devEspr.timeout))
}

func (devEspr *DeviceEspresso) NewPress() engine.Doer {
	tag := devEspr.name + ".press"
	return engine.NewSeq(tag).
		Append(devEspr.Generic.NewWaitReady(tag)).
		Append(devEspr.Generic.NewAction(tag, 0x02)).
		Append(devEspr.Generic.NewWaitDone(tag, devEspr.timeout))
}

func (devEspr *DeviceEspresso) NewRelease() engine.Doer {
	tag := devEspr.name + ".release"
	return engine.NewSeq(tag).
		Append(devEspr.Generic.NewWaitReady(tag)).
		Append(devEspr.Generic.NewAction(tag, 0x03)).
		Append(devEspr.Generic.NewWaitDone(tag, devEspr.timeout))
}

func (devEspr *DeviceEspresso) NewHeat(on bool) engine.Doer {
	tag := fmt.Sprintf("%s.heat:%t", devEspr.name, on)
	arg := byte(0x05)
	if !on {
		arg = 0x06
	}
	return engine.NewSeq(tag).
		Append(devEspr.Generic.NewWaitReady(tag)).
		Append(devEspr.Generic.NewAction(tag, arg))
}
