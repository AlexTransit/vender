package evend

import (
	"context"
	"strconv"

	"github.com/AlexTransit/vender/internal/state"
)

func EnumHopper(ctx context.Context, hNo int) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceHopper{}
	suffix := strconv.Itoa(hNo)
	dev.name = "hopper" + suffix
	addr := uint8(0x40 + (hNo-1)*8)
	return g.RegisterDevice("evend."+dev.name, dev, func() error {
		err := dev.init(ctx, addr, proto2)
		return err
		// return dev.init(ctx, addr, suffix, proto2)
	})
}

func EnumMultiHopper(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceHopper{}
	dev.name = "multihopper"
	return g.RegisterDevice("evend."+dev.name, dev, func() error {
		err := dev.init(ctx, 0xb8, proto1)
		return err
		// return dev.init(ctx, 0xb8, "", proto1)
	})
}

func EnumValve(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	// dev := &DeviceValve{}
	dev := &DeviceValve{}
	return g.RegisterDevice("evend.valve", dev, func() error { return dev.init(ctx) })
}

func EnumMixer(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceMixer{}
	return g.RegisterDevice("evend.mixer", dev, func() error { return dev.init(ctx) })
}

func EnumEspresso(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceEspresso{}
	return g.RegisterDevice("evend.espresso", dev, func() error { return dev.init(ctx) })
}

func EnumElevator(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceElevator{}
	return g.RegisterDevice("evend.elevator", dev, func() error { return dev.init(ctx) })
}

func EnumCup(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceCup{}
	return g.RegisterDevice("evend.cup", dev, func() error { return dev.init(ctx) })
}

func EnumConveyor(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceConveyor{}
	return g.RegisterDevice("evend.conveyor", dev, func() error { return dev.init(ctx) })
}
