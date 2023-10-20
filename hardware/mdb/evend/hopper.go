package evend

import (
	"context"
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
		return runWitchControl(h, byte(spinTime), 0)
	})
	g.Engine.RegisterNewFunc(h.name+".reset", func(ctx context.Context) error { return h.reset() })
	return h.reset()
}

func (mh *DeviceMultiHopper) init(ctx context.Context) error {
	const addr = 0xb8
	g := state.GetGlobal(ctx)
	mh.Generic.Init(ctx, addr, "multihopper", proto1)

	g.Engine.RegisterNewFunc(mh.name+".reset", func(ctx context.Context) error { return mh.reset() })
	for i := uint8(1); i <= 10; i++ {
		hopperNumber := i
		g.Engine.RegisterNewFuncAgr(fmt.Sprintf("%s%d.run(?)", mh.name, hopperNumber), func(ctx context.Context, spinTime engine.Arg) (err error) {
			return runWitchControl(mh, byte(spinTime), hopperNumber)
		})
	}
	return mh.reset()
}

func runWitchControl(b BunkerDevice, spinTime byte, hopperNumber byte) (err error) {
	if spinTime == 0 {
		return
	}
	if err = b.run(spinTime, hopperNumber); err == nil {
		return
	}
	b.reset()
	time.Sleep(5 * time.Second)
	if e := b.run(byte(spinTime), hopperNumber); e != nil {
		b.reset()
		return fmt.Errorf("two times errors error1(%v) error2(%v)", err, e)
	}
	b.logError(fmt.Errorf("restart fix error (%v)", err))
	return
}

func (h *DeviceHopper) run(spinTime byte, _tmp byte) (err error) {
	timeout := uint16(spinTime) + 5
	return h.CommandWaitSuccess(timeout, spinTime)
}

func (mh *DeviceMultiHopper) run(spinTime byte, hopperNumber byte) (err error) {
	timeout := uint16(spinTime) + 5
	return mh.CommandWaitSuccess(timeout, hopperNumber, spinTime)
}
