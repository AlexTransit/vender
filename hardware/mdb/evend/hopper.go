package evend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

type DeviceHopper struct {
	Generic
}

type DeviceMultiHopper struct {
	Generic
}
type BunkerDevice interface {
	run(byte, byte) error
	reset() error
	logError(error)
}

func (h *DeviceHopper) reset() error             { return h.dev.Rst() }
func (h *DeviceHopper) logError(err error)       { h.dev.Log.Error(err) }
func (mh *DeviceMultiHopper) reset() error       { return mh.dev.Rst() }
func (mh *DeviceMultiHopper) logError(err error) { mh.dev.Log.Error(err) }

func (h *DeviceHopper) init(ctx context.Context, addr uint8, nameSuffix string) error {
	name := "hopper" + nameSuffix
	g := state.GetGlobal(ctx)
	h.Generic.Init(ctx, addr, name, proto2)
	g.Engine.RegisterNewFuncAgr(h.name+".run(?)", func(ctx context.Context, spinTime engine.Arg) (err error) {
		return runWitchControl(h, byte(spinTime.(int16)), 0)
	})
	g.Engine.RegisterNewFunc(h.name+".reset", func(ctx context.Context) error { return h.reset() })
	return h.reset()
}

func (mh *DeviceMultiHopper) init(ctx context.Context) error {
	const addr = 0xb8
	g := state.GetGlobal(ctx)
	mh.Generic.Init(ctx, addr, "multihopper", proto1)

	g.Engine.RegisterNewFunc(mh.name+".reset", func(ctx context.Context) error { return mh.reset() })
	for i := uint8(1); i <= 8; i++ {
		hopperNumber := i
		g.Engine.RegisterNewFuncAgr(fmt.Sprintf("%s%d.run(?)", mh.name, hopperNumber), func(ctx context.Context, spinTime engine.Arg) (err error) {
			return runWitchControl(mh, byte(spinTime.(int16)), hopperNumber)
		})
	}
	return mh.reset()
}

func runWitchControl(b BunkerDevice, spinTime byte, hopperNumber byte) (err error) {
	if spinTime == 0 {
		return
	}
	for i := 1; i <= 2; i++ {
		e := b.run(spinTime, hopperNumber)
		if e == nil {
			if err != nil {
				b.logError(fmt.Errorf("(%d) restart fix error (%v)", i, err))
			}
			return
		}
		err = errors.Join(err, e)
		b.reset()
		time.Sleep(5 * time.Second)
	}
	return err
}

func (h *DeviceHopper) run(spinTime byte, _tmp byte) error {
	h.dev.Action = fmt.Sprintf("hopper %s run(%v)", h.name, spinTime)
	timeout := uint16(spinTime) + 5
	h.log.Infof("%s start (%d)", h.name, spinTime)
	defer h.log.Infof("%s stop", h.name)
	return h.CommandWaitSuccess(timeout, spinTime)
}

func (mh *DeviceMultiHopper) run(spinTime byte, hopperNumber byte) error {
	mh.dev.Action = fmt.Sprintf("multihopper%v run(%v)", hopperNumber, spinTime)
	timeout := uint16(spinTime) + 5
	mh.log.Infof("%s%d start (%d)", mh.name, hopperNumber, spinTime)
	defer mh.log.Infof("%s%d stop", mh.name, hopperNumber)
	return mh.CommandWaitSuccess(timeout, hopperNumber, spinTime)
}
