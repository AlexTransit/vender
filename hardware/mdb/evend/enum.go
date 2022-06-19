package evend

import (
	"context"
	"strconv"

	"github.com/AlexTransit/vender/internal/state"
)

func EnumHopper(ctx context.Context, hNo int) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceHopper{}
	addr := uint8(0x40 + (hNo-1)*8)
	suffix := strconv.Itoa(hNo)
	return g.RegisterDevice("evend.hopper"+suffix, dev, func() error { return dev.init(ctx, addr, suffix) })
}

func EnumMultiHopper(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	dev := &DeviceMultiHopper{}
	return g.RegisterDevice("evend.multihopper", dev, func() error { return dev.init(ctx) })
}

func EnumValve(ctx context.Context) error {
	g := state.GetGlobal(ctx)
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
