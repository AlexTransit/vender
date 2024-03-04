package evend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
)

const defaultEspressoTimeout = 30

type DeviceEspresso struct {
	Generic

	timeout uint16
}

func (d *DeviceEspresso) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	d.timeout = uint16(helpers.ConfigDefaultInt(g.Config.Hardware.Evend.Espresso.TimeoutSec, defaultEspressoTimeout)) * 5 // every 200 ms
	d.Generic.Init(ctx, 0xe8, "espresso", proto2)
	g.Engine.RegisterNewFunc(d.name+".waitDone", func(ctx context.Context) error { return d.Proto2PollWaitSuccess(d.timeout, true, false) })
	g.Engine.RegisterNewFunc(d.name+".grindNoWait", func(ctx context.Context) error { return d.grindNoWait() })
	g.Engine.RegisterNewFunc(d.name+".grind", func(ctx context.Context) error { return d.grind() })
	g.Engine.RegisterNewFunc(d.name+".pressNoWait", func(ctx context.Context) error { return d.pressNoWait() })
	g.Engine.RegisterNewFunc(d.name+".press", func(ctx context.Context) error { return d.press() })
	g.Engine.RegisterNewFunc(d.name+".disposeNoWait", func(ctx context.Context) error { return d.releaseNoWait() })
	g.Engine.RegisterNewFunc(d.name+".dispose", func(ctx context.Context) error { return d.release() })
	g.Engine.RegisterNewFunc(d.name+".heat_on", func(ctx context.Context) error { return d.heatOn() })
	g.Engine.RegisterNewFunc(d.name+".heat_off", func(ctx context.Context) error { return d.heatOff() })
	g.Engine.RegisterNewFunc(d.name+".reset", func(ctx context.Context) error { return d.dev.Rst() })

	if err := d.dev.Rst(); err != nil {
		return fmt.Errorf("init %s:%v", d.name, err)
	}
	return nil
}

func (d *DeviceEspresso) grindNoWait() (err error) {
	for i := 0; i < 5; i++ {
		e := d.CommandNoWait(0x01)
		if e == nil {
			if err != nil {
				d.dev.Log.Errf("%d restart fix problem (%v)", i, err)
			}
			return nil
		}
		err = errors.Join(err, e)
		time.Sleep(5 * time.Second)
	}
	return err
}

func (d *DeviceEspresso) pressNoWait() error   { return d.CommandNoWait(0x02) }
func (d *DeviceEspresso) releaseNoWait() error { return d.CommandNoWait(0x03) }
func (d *DeviceEspresso) heatOn() error        { return d.CommandNoWait(0x05) }
func (d *DeviceEspresso) heatOff() error       { return d.CommandNoWait(0x06) }

func (d *DeviceEspresso) grind() (err error) {
	if err = d.grindNoWait(); err != nil {
		return
	}
	return d.WaitSuccess(d.timeout, true)
}

func (d *DeviceEspresso) press() (err error) {
	if err = d.pressNoWait(); err != nil {
		return
	}
	return d.WaitSuccess(d.timeout, true)
}

func (d *DeviceEspresso) release() (err error) {
	if err = d.releaseNoWait(); err != nil {
		return
	}
	return d.WaitSuccess(d.timeout, true)
}
