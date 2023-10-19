package evend

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

const DefaultHopperRunTimeout = 200 * time.Millisecond
const HopperTimeout = 1 * time.Second

type DeviceHopper struct {
	Generic
}

type DeviceMultiHopper struct {
	Generic
}

func (h *DeviceHopper) init(ctx context.Context, addr uint8, nameSuffix string) error {
	name := "hopper" + nameSuffix
	g := state.GetGlobal(ctx)
	h.Generic.Init(ctx, addr, name, proto2)
	g.Engine.RegisterNewFuncAgr(h.name+".run(?)", func(ctx context.Context, spinTime engine.Arg) (err error) {
		if spinTime == 0 {
			return
		}
		if err = h.run(byte(spinTime)); err == nil {
			return
		}
		h.dev.Rst()
		time.Sleep(5 * time.Second)
		if e := h.run(byte(spinTime)); e != nil {
			return fmt.Errorf("two times errors e1(%v) e2(%v)", err, e)
		}
		h.dev.Log.Errorf("restart fix error (%v)", err)
		return nil
	})
	g.Engine.RegisterNewFunc(h.name+".reset", func(ctx context.Context) error {
		return h.dev.Rst()
	})
	err := h.dev.Rst()
	return err
}

func (mh *DeviceMultiHopper) init(ctx context.Context) error {
	const addr = 0xb8
	g := state.GetGlobal(ctx)
	mh.Generic.Init(ctx, addr, "multihopper", proto1)

	g.Engine.RegisterNewFunc(mh.name+".reset", func(ctx context.Context) error {
		return mh.dev.Rst()
	})
	for i := uint8(1); i <= 10; i++ {
		hopperNumber := i
		g.Engine.RegisterNewFuncAgr(fmt.Sprintf("%s%d.run(?)", mh.name, hopperNumber), func(ctx context.Context, spinTime engine.Arg) (err error) {
			if spinTime == 0 {
				return
			}
			if err = mh.run(byte(spinTime), hopperNumber); err == nil {
				return nil
			}
			mh.dev.Rst()
			time.Sleep(5 * time.Second)
			if e := mh.run(byte(spinTime), hopperNumber); e != nil {
				mh.dev.Rst()
				return fmt.Errorf("two times errors e1(%v) e2(%v)", err, e)
			}
			mh.dev.Log.Errorf("restart fix error (%v)", err)
			return nil
		})
	}
	return mh.dev.Rst()
}

func (h *DeviceHopper) run(spinTime byte) (err error) {
	timeout := uint16(spinTime) * 6
	if err = h.CommandWaitSuccess(timeout, spinTime); err != nil {
		return err
	}
	return h.ReadError()
}

func (mh *DeviceMultiHopper) run(spinTime byte, hopperNumber byte) (err error) {
	timeout := uint16(spinTime) * 6
	return mh.CommandWaitSuccess(timeout, hopperNumber, spinTime)
}
