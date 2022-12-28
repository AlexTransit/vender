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

func (d *DeviceEspresso) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	espressoConfig := &g.Config.Hardware.Evend.Espresso
	d.timeout = helpers.IntSecondDefault(espressoConfig.TimeoutSec, DefaultEspressoTimeout)
	d.Generic.Init(ctx, 0xe8, "espresso", proto2)

	g.Engine.Register(d.name+".grind", engine.Func{F: func(ctx context.Context) (err error) {
		for i := 0; i < 3; i++ {
			if err = g.Engine.Exec(ctx, d.grind()); err == nil {
				if i != 0 {
					d.dev.TeleError(errors.Errorf("restart fix preview error"))
				}
				return nil
			}
			d.dev.TeleError(err)
			d.dev.Reset()
		}
		d.dev.TeleError(errors.Annotatef(err, "many times error"))
		return err
	}})
	g.Engine.Register(d.name+".grindNoWait", d.GrindNoWait())
	g.Engine.Register(d.name+".waitDone", d.WaitDone())
	g.Engine.Register(d.name+".press", d.NewPress())
	g.Engine.Register(d.name+".dispose", d.NewRelease())
	g.Engine.Register(d.name+".heat_on", d.NewHeat(true))
	g.Engine.Register(d.name+".heat_off", d.NewHeat(false))

	err := d.Generic.FIXME_initIO(ctx)
	return errors.Annotate(err, d.name+".init")
}

func (d *DeviceEspresso) grind() engine.Doer {
	tag := d.name + ".grind"
	return engine.NewSeq(tag).
		Append(d.Generic.NewWaitReady(tag)).
		Append(d.Generic.NewAction(tag, 0x01)).
		// TODO expect delay like in cup dispense, ignore immediate error, retry
		Append(d.Generic.NewWaitDone(tag, d.timeout))
}

func (d *DeviceEspresso) NewGrind() engine.Doer {
	tag := d.name + ".grind"
	return engine.NewSeq(tag).
		Append(d.Generic.NewWaitReady(tag)).
		Append(d.Generic.NewAction(tag, 0x01)).
		// TODO expect delay like in cup dispense, ignore immediate error, retry
		Append(d.Generic.NewWaitDone(tag, d.timeout))
}

func (d *DeviceEspresso) GrindNoWait() engine.Doer {
	tag := d.name + ".grindNoWait"
	return engine.NewSeq(tag).
		Append(d.Generic.NewAction(tag, 0x01))
}

func (d *DeviceEspresso) WaitDone() engine.Doer {
	tag := d.name + ".waitDone"
	return engine.NewSeq(tag).
		Append(d.Generic.NewWaitReady(tag)).
		Append(d.Generic.NewWaitDone(tag, d.timeout))
}

func (d *DeviceEspresso) NewPress() engine.Doer {
	tag := d.name + ".press"
	return engine.NewSeq(tag).
		Append(d.Generic.NewWaitReady(tag)).
		Append(d.Generic.NewAction(tag, 0x02)).
		Append(d.Generic.NewWaitDone(tag, d.timeout))
}

func (d *DeviceEspresso) NewRelease() engine.Doer {
	tag := d.name + ".release"
	return engine.NewSeq(tag).
		Append(d.Generic.NewWaitReady(tag)).
		Append(d.Generic.NewAction(tag, 0x03)).
		Append(d.Generic.NewWaitDone(tag, d.timeout))
}

func (d *DeviceEspresso) NewHeat(on bool) engine.Doer {
	tag := fmt.Sprintf("%s.heat:%t", d.name, on)
	arg := byte(0x05)
	if !on {
		arg = 0x06
	}
	return engine.NewSeq(tag).
		Append(d.Generic.NewWaitReady(tag)).
		Append(d.Generic.NewAction(tag, arg))
}
