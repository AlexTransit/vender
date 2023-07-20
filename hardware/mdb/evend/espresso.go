package evend

import (
	"context"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
)

const defaultEspressoTimeout = 30

type DeviceEspresso struct {
	Generic

	timeout uint8
}

func (d *DeviceEspresso) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	espressoConfig := &g.Config.Hardware.Evend.Espresso
	d.timeout = uint8(helpers.IntConfigDefault(espressoConfig.TimeoutSec, defaultEspressoTimeout)) * 5 //every 200 ms
	d.Generic.Init(ctx, 0xe8, "espresso", proto2)
	g.Engine.RegisterNewFunc(d.name+".waitDone", func(ctx context.Context) error { return d.Proto2PollWaitSuccess(100) })
	g.Engine.RegisterNewFunc(d.name+".grindNoWait", func(ctx context.Context) error { return d.grindNoWait() })
	g.Engine.RegisterNewFunc(d.name+".grind", func(ctx context.Context) error { return d.grind() })
	g.Engine.RegisterNewFunc(d.name+".pressNoWait", func(ctx context.Context) error { return d.pressNoWait() })
	g.Engine.RegisterNewFunc(d.name+".press", func(ctx context.Context) error { return d.press() })
	g.Engine.RegisterNewFunc(d.name+".disposeNoWait", func(ctx context.Context) error { return d.releaseNoWait() })
	g.Engine.RegisterNewFunc(d.name+".dispose", func(ctx context.Context) error { return d.release() })
	g.Engine.RegisterNewFunc(d.name+".heat_on", func(ctx context.Context) error { return d.heatOn() })
	g.Engine.RegisterNewFunc(d.name+".heat_off", func(ctx context.Context) error { return d.heatOff() })

	err := d.dev.Rst()
	time.Sleep(200 * time.Millisecond)
	return errors.Annotate(err, d.name+".init")
}

func (d *DeviceEspresso) pollBeforeAfter(f func()) error {
	if err := d.Proto2PollWaitSuccess(5); err != nil {
		return err
	}
	f()
	return d.Proto2PollWaitSuccess(5)
}

func (d *DeviceEspresso) cmd(byteCmd byte) error {
	return d.pollBeforeAfter(func() { d.Command([]byte{byteCmd}) })
}

func (d *DeviceEspresso) grindNoWait() error {
	d.Proto2PollWaitSuccess(5)
	d.cmd(0x05)
	return d.cmd(0x01)
}

func (d *DeviceEspresso) grind() (err error) {
	if err = d.grindNoWait(); err != nil {
		return
	}
	return d.Proto2PollWaitSuccess(d.timeout)
}

func (d *DeviceEspresso) pressNoWait() error {
	return d.cmd(0x02)
}

func (d *DeviceEspresso) press() (err error) {
	if err = d.pressNoWait(); err != nil {
		return
	}
	return d.Proto2PollWaitSuccess(d.timeout)
}

func (d *DeviceEspresso) releaseNoWait() error {
	return d.cmd(0x03)
}

func (d *DeviceEspresso) release() (err error) {
	if err = d.releaseNoWait(); err != nil {
		return
	}
	return d.Proto2PollWaitSuccess(d.timeout)
}

func (d *DeviceEspresso) heatOn() error {
	return d.cmd(0x05)
}

func (d *DeviceEspresso) heatOff() error {
	return d.cmd(0x06)
}
