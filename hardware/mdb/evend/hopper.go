package evend

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

type DeviceHopper struct {
	Generic
	count uint8
}

type BunkerDevice interface {
	run(byte, byte) error
	reset() error
	// logError(error)
}

func (h *DeviceHopper) reset() error { return h.dev.Rst() }

// func (h *DeviceHopper) logError(err error) { h.dev.Log.Error(err) }

func (h *DeviceHopper) init(ctx context.Context, addr uint8, proto evendProtocol) error {
	g := state.GetGlobal(ctx)
	h.Generic.Init(ctx, addr, h.name, proto)

	g.Engine.RegisterNewFunc(h.name+".reset", func(ctx context.Context) error { return h.dev.Rst() })
	// одиночный хоппер
	if proto == proto2 {
		g.Engine.RegisterNewFuncAgr(h.name+".run(?)", func(ctx context.Context, spinTime engine.Arg) error {
			return runWithControl(h, byte(spinTime.(int16)), 0)
		})
		g.Engine.RegisterNewFuncAgr(h.name+".runNoWait(?)", func(ctx context.Context, spinTime engine.Arg) error {
			return runNoWait(h, byte(spinTime.(int16)), 0)
		})
	}
	// мультихоппер
	if proto == proto1 {
		for i := uint8(1); i <= 8; i++ {
			n := i
			g.Engine.RegisterNewFuncAgr(fmt.Sprintf("%s%d.run(?)", h.name, n), func(ctx context.Context, spinTime engine.Arg) error {
				return runWithControl(h, byte(spinTime.(int16)), n)
			})
			g.Engine.RegisterNewFuncAgr(fmt.Sprintf("%s%d.runNoWait(?)", h.name, n), func(ctx context.Context, spinTime engine.Arg) error {
				return runNoWait(h, byte(spinTime.(int16)), n)
			})
		}
	}
	return h.dev.Rst()
}

func (h *DeviceHopper) run(spinTime byte, hopperNumber byte) error {
	if hopperNumber == 0 {
		h.dev.Action = fmt.Sprintf("hopper %s run(%v)", h.name, spinTime)
		h.log.Infof("%s start (%d)", h.name, spinTime)
		defer h.log.Infof("%s stop", h.name)
		return h.CommandWaitSuccess(uint16(spinTime)+5, spinTime)
	}
	h.dev.Action = fmt.Sprintf("multihopper%v run(%v)", hopperNumber, spinTime)
	h.log.Infof("%s%d start (%d)", h.name, hopperNumber, spinTime)
	defer h.log.Infof("%s%d stop", h.name, hopperNumber)
	return h.CommandWaitSuccess(uint16(spinTime)+5, hopperNumber, spinTime)
}

func runWithControl(b BunkerDevice, spinTime byte, hopperNumber byte) error {
	if spinTime == 0 {
		return nil
	}
	return b.run(spinTime, hopperNumber)
}

func runNoWait(b BunkerDevice, spinTime byte, hopperNumber byte) error {
	if spinTime == 0 {
		return nil
	}
	return b.run(spinTime, hopperNumber)
}

func (h *DeviceHopper) runNoWait(spinTime byte, hopperNumber byte) error {
	if hopperNumber == 0 {
		h.log.Infof("%s start (%d)", h.name, spinTime)
		return h.CommandNoWait(spinTime)
	}
	h.log.Infof("%s%d start (%d)", h.name, hopperNumber, spinTime)
	return h.CommandNoWait(hopperNumber, spinTime)
}
