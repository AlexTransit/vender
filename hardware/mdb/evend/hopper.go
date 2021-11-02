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

const DefaultHopperRunTimeout = 200 * time.Millisecond
const HopperTimeout = 1 * time.Second

type DeviceHopper struct {
	Generic
}

func (devHop *DeviceHopper) init(ctx context.Context, addr uint8, nameSuffix string) error {
	name := "hopper" + nameSuffix
	g := state.GetGlobal(ctx)
	devHop.Generic.Init(ctx, addr, name, proto2)

	do := newHopperRun(&devHop.Generic, fmt.Sprintf("%s.run", devHop.name), nil)
	g.Engine.Register(fmt.Sprintf("%s.run(?)", devHop.name), do)

	err := devHop.Generic.FIXME_initIO(ctx)
	return errors.Annotate(err, devHop.name+".init")
}

type DeviceMultiHopper struct {
	Generic
}

func (devHop *DeviceMultiHopper) init(ctx context.Context) error {
	const addr = 0xb8
	g := state.GetGlobal(ctx)
	devHop.Generic.Init(ctx, addr, "multihopper", proto1)

	for i := uint8(1); i <= 8; i++ {
		do := newHopperRun(
			&devHop.Generic,
			fmt.Sprintf("%s%d.run", devHop.name, i),
			[]byte{i},
		)
		g.Engine.Register(fmt.Sprintf("%s%d.run(?)", devHop.name, i), do)
	}

	err := devHop.Generic.FIXME_initIO(ctx)
	return errors.Annotate(err, devHop.name+".init")
}

func newHopperRun(gen *Generic, tag string, argsPrefix []byte) engine.FuncArg {
	return engine.FuncArg{Name: tag, F: func(ctx context.Context, arg engine.Arg) error {
		g := state.GetGlobal(ctx)
		hopperConfig := &g.Config.Hardware.Evend.Hopper
		units := uint8(arg)
		runTimeout := helpers.IntMillisecondDefault(hopperConfig.RunTimeoutMs, DefaultHopperRunTimeout)

		if err := g.Engine.Exec(ctx, gen.NewWaitReady(tag)); err != nil {
			return err
		}
		args := append(argsPrefix, units)
		if err := gen.txAction(args); err != nil {
			return err
		}
		return g.Engine.Exec(ctx, gen.NewWaitDone(tag, runTimeout*time.Duration(units)+HopperTimeout))
	}}
}
