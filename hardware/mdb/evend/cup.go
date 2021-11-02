package evend

import (
	"context"
	"fmt"

	// "go/types"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/juju/errors"
)

const DefaultCupAssertBusyDelay = 30 * time.Millisecond
const DefaultCupDispenseTimeout = 10 * time.Second
const DefaultCupEnsureTimeout = 70 * time.Second

type DeviceCup struct {
	Generic
	dispenseTimeout     time.Duration
	assertBusyDelayMils time.Duration
}

func (devCup *DeviceCup) init(ctx context.Context) error {
	devCup.Generic.Init(ctx, 0xe0, "cup", proto2)

	g := state.GetGlobal(ctx)
	doDispense := devCup.Generic.WithRestart(devCup.NewDispenseProper())
	g.Engine.Register(devCup.name+".dispense", doDispense)
	g.Engine.Register(devCup.name+".light_on", devCup.NewLight(true))
	g.Engine.Register(devCup.name+".light_off", devCup.NewLight(false))
	g.Engine.Register(devCup.name+".ensure", devCup.NewEnsure())
	devCup.dispenseTimeout = helpers.IntSecondDefault(g.Config.Hardware.Evend.Cup.DispenseTimeoutSec, DefaultCupDispenseTimeout)
	devCup.assertBusyDelayMils = helpers.IntMillisecondDefault(g.Config.Hardware.Evend.Cup.AssertBusyDelayMs, DefaultCupAssertBusyDelay)

	err := devCup.Generic.FIXME_initIO(ctx)
	return errors.Annotate(err, devCup.name+".init")
}

func (devCup *DeviceCup) NewDispenseProper() engine.Doer {
	return engine.NewSeq(devCup.name + ".dispense_proper").
		Append(devCup.NewEnsure()).
		Append(devCup.NewDispense())
}

func (devCup *DeviceCup) NewDispense() engine.Doer {
	tag := devCup.name + ".dispense"
	return engine.NewSeq(tag).
		Append(engine.Func0{F: func() error { types.Log.Info("cup dispence"); return nil }}).
		Append(devCup.Generic.NewWaitReady(tag)).
		Append(devCup.Generic.NewAction(tag, 0x01)).
		Append(engine.Func{Name: tag + "/assert-busy", F: func(ctx context.Context) error {
			time.Sleep(devCup.assertBusyDelayMils)
			response := mdb.Packet{}
			err := devCup.dev.TxKnown(devCup.dev.PacketPoll, &response)
			if err != nil {
				return err
			}
			bs := response.Bytes()
			if len(bs) != 1 {
				return devCup.NewErrPollUnexpected(response)
			}
			if bs[0] != devCup.proto2BusyMask {
				devCup.dev.Log.Errorf("expected BUSY, cup device is broken")
				return devCup.NewErrPollUnexpected(response)
			}
			return nil
		}}).
		Append(engine.Func{
			F: func(ctx context.Context) error {
				g := state.GetGlobal(ctx)
				return g.Engine.Exec(ctx, devCup.Generic.NewWaitDone(tag, devCup.dispenseTimeout))
			},
		})
}

func (devCup *DeviceCup) NewLight(v bool) engine.Doer {
	tag := fmt.Sprintf("%s.light:%t", devCup.name, v)
	arg := byte(0x02)
	if !v {
		arg = 0x03
	}
	types.SetLight(v)
	// return devCup.Generic.NewAction(tag, arg)
	return engine.NewSeq(tag).
		Append(devCup.Generic.NewAction(tag, arg)).
		Append(engine.Func0{F: func() error { types.SetLight(v); return nil }})

}

func (devCup *DeviceCup) NewEnsure() engine.Doer {
	tag := devCup.name + ".ensure"
	return engine.NewSeq(tag).
		Append(devCup.Generic.NewWaitReady(tag)).
		Append(devCup.Generic.NewAction(tag, 0x04)).
		Append(engine.Func{
			F: func(ctx context.Context) error {
				g := state.GetGlobal(ctx)
				cupConfig := &g.Config.Hardware.Evend.Cup
				ensureTimeout := helpers.IntSecondDefault(cupConfig.EnsureTimeoutSec, DefaultCupEnsureTimeout)
				return g.Engine.Exec(ctx, devCup.Generic.NewWaitDone(tag, ensureTimeout))
			},
		})
}
