package evend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/log2"
)

type DeviceEspresso struct {
	Generic
	timeout uint16
	log     log2.Log
}

func (d *DeviceEspresso) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	d.timeout = uint16(g.Config.Hardware.Evend.Espresso.TimeoutSec)
	d.Generic.Init(ctx, 0xe8, "espresso", proto2)
	d.log = *g.Log
	g.Engine.RegisterNewFunc(d.name+".waitDone", func(ctx context.Context) error { return d.Proto2PollWaitSuccess(d.timeout, true) })
	// g.Engine.RegisterNewFunc(d.name+".grindNoWait", func(ctx context.Context) error { return d.grindNoWait() })
	g.Engine.RegisterNewFuncAgr(d.name+".grindNoWait(?)", func(ctx context.Context, _ engine.Arg) error { return d.grindNoWait() })
	// g.Engine.RegisterNewFunc(d.name+".grind", func(ctx context.Context) error { return d.grind() })
	g.Engine.RegisterNewFuncAgr(d.name+".grind(?)", func(ctx context.Context, _ engine.Arg) error { return d.grind() })
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

func (d *DeviceEspresso) grindNoWait() error { return d.CommandNoWait(0x01) }

func (d *DeviceEspresso) pressNoWait() error   { return d.CommandNoWait(0x02) }
func (d *DeviceEspresso) releaseNoWait() error { return d.CommandNoWait(0x03) }
func (d *DeviceEspresso) heatOn() error        { return d.CommandNoWait(0x05) }
func (d *DeviceEspresso) heatOff() error       { return d.CommandNoWait(0x06) }

func (d *DeviceEspresso) grind() (err error) {
	for i := 0; i < 5; i++ {
		d.log.Debug("grind start")
		e := d.CommandWaitSuccess(d.timeout, 0x01)
		if e == nil {
			if i > 0 {
				// d.log.WarningF("%d restart fix problem (%v)", i, err)
				d.dev.TeleError(fmt.Errorf("%d restart fix problem (%v)", i, err))
			}
			d.log.Debug("grind complete")
			return nil
		}
		d.log.WarningF("grind error (%v)", e)
		err = errors.Join(err, e)
		time.Sleep(5 * time.Second)
	}
	d.log.Errorf("grind not complete (%v)", err)
	return err
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
