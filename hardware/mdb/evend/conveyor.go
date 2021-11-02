package evend

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
)

const ConveyorDefaultTimeout = 30 * time.Second
const ConveyorMinTimeout = 1 * time.Second

type DeviceConveyor struct { //nolint:maligned
	Generic

	DoSetSpeed engine.FuncArg
	maxTimeout time.Duration
	minSpeed   uint16
	currentPos int16 // estimated
}

func (devConv *DeviceConveyor) init(ctx context.Context) error {
	devConv.currentPos = -1
	g := state.GetGlobal(ctx)
	devConfig := &g.Config.Hardware.Evend.Conveyor
	keepaliveInterval := helpers.IntMillisecondDefault(devConfig.KeepaliveMs, 0)
	devConv.minSpeed = uint16(devConfig.MinSpeed)
	if devConv.minSpeed == 0 {
		devConv.minSpeed = 200
	}
	devConv.maxTimeout = speedDistanceDuration(float32(devConv.minSpeed), uint(devConfig.PositionMax))
	if devConv.maxTimeout == 0 {
		devConv.maxTimeout = ConveyorDefaultTimeout
	}
	g.Log.Debugf("evend.conveyor minSpeed=%d maxTimeout=%v keepalive=%v", devConv.minSpeed, devConv.maxTimeout, keepaliveInterval)
	devConv.dev.DelayNext = 245 * time.Millisecond // empirically found lower total WaitReady
	devConv.Generic.Init(ctx, 0xd8, "conveyor", proto2)
	devConv.DoSetSpeed = devConv.newSetSpeed()

	doCalibrate := engine.Func{Name: devConv.name + ".calibrate", F: devConv.calibrate}
	doMove := engine.FuncArg{
		Name: devConv.name + ".move",
		F: func(ctx context.Context, arg engine.Arg) error {
			return devConv.move(ctx, uint16(arg))
		}}
	moveSeq := engine.NewSeq(devConv.name + ".move(?)").Append(doCalibrate).Append(doMove)
	g.Engine.Register(moveSeq.String(), devConv.Generic.WithRestart(moveSeq))
	g.Engine.Register(devConv.name+".set_speed(?)", devConv.DoSetSpeed)

	doShake := engine.FuncArg{
		Name: devConv.name + ".shake",
		F: func(ctx context.Context, arg engine.Arg) error {
			return devConv.shake(ctx, uint8(arg))
		}}
	g.Engine.RegisterNewSeq(devConv.name+".shake(?)", doCalibrate, doShake)

	err := devConv.Generic.FIXME_initIO(ctx)
	if keepaliveInterval > 0 {
		go devConv.Generic.dev.Keepalive(keepaliveInterval, g.Alive.StopChan())
	}
	return errors.Annotate(err, devConv.name+".init")
}

func (devConv *DeviceConveyor) calibrate(ctx context.Context) error {
	// devConv.dev.Log.Debugf("%s calibrate ready=%t current=%d", devConv.name, devConv.dev.Ready(), devConv.currentPos)
	if devConv.currentPos >= 0 {
		return nil
	}
	// devConv.dev.Log.Debugf("%s calibrate begin", devConv.name)
	err := devConv.move(ctx, 0)
	if err == nil {
		devConv.dev.Log.Debugf("%s calibrate success", devConv.name)
	}
	return errors.Annotate(err, devConv.name+".calibrate")
}

func (devConv *DeviceConveyor) move(ctx context.Context, position uint16) error {
	g := state.GetGlobal(ctx)
	tag := fmt.Sprintf("%s.move:%d", devConv.name, position)
	tbegin := time.Now()
	if g.Config.Hardware.Evend.Conveyor.LogDebug {
		devConv.dev.Log.Debugf("%s begin", tag)
	}

	doWaitDone := engine.Func{F: func(ctx context.Context) error {
		timeout := devConv.maxTimeout
		if devConv.dev.Ready() && devConv.currentPos >= 0 {
			distance := absDiffU16(uint16(devConv.currentPos), position)
			eta := speedDistanceDuration(float32(devConv.minSpeed), uint(distance))
			timeout = eta * 2
		}
		if timeout < ConveyorMinTimeout {
			timeout = ConveyorMinTimeout
		}
		devConv.dev.Log.Debugf("%s position current=%d target=%d timeout=%v maxtimeout=%v", tag, devConv.currentPos, position, timeout, devConv.maxTimeout)

		err := g.Engine.Exec(ctx, devConv.Generic.NewWaitDone(tag, timeout))
		if err != nil {
			devConv.currentPos = -1
			// TODO check SetReady(false)
		} else {
			devConv.currentPos = int16(position)
			devConv.dev.SetReady()
			if g.Config.Hardware.Evend.Conveyor.LogDebug {
				devConv.dev.Log.Debugf("%s duration=%s", tag, time.Since(tbegin))
			}
		}
		return err
	}}

	// TODO engine InlineSeq
	seq := engine.NewSeq(tag).
		Append(devConv.Generic.NewWaitReady(tag)).
		Append(devConv.Generic.NewAction(tag, 0x01, byte(position&0xff), byte(position>>8))).
		Append(doWaitDone)
	err := g.Engine.Exec(ctx, seq)
	return errors.Annotate(err, tag)
}

func (devConv *DeviceConveyor) shake(ctx context.Context, arg uint8) error {
	g := state.GetGlobal(ctx)
	tag := fmt.Sprintf("%s.shake:%d", devConv.name, arg)

	doWaitDone := engine.Func{F: func(ctx context.Context) error {
		err := g.Engine.Exec(ctx, devConv.Generic.NewWaitDone(tag, devConv.maxTimeout))
		if err != nil {
			devConv.currentPos = -1
			// TODO check SetReady(false)
		}
		return err
	}}

	// TODO engine InlineSeq
	seq := engine.NewSeq(tag).
		Append(devConv.Generic.NewWaitReady(tag)).
		Append(devConv.Generic.NewAction(tag, 0x03, byte(arg), 0)).
		Append(doWaitDone)
	err := g.Engine.Exec(ctx, seq)
	return errors.Annotate(err, tag)
}

func (devConv *DeviceConveyor) newSetSpeed() engine.FuncArg {
	tag := devConv.name + ".set_speed"

	return engine.FuncArg{Name: tag, F: func(ctx context.Context, arg engine.Arg) error {
		speed := uint8(arg)
		bs := []byte{devConv.dev.Address + 5, 0x10, speed}
		request := mdb.MustPacketFromBytes(bs, true)
		response := mdb.Packet{}
		err := devConv.dev.TxCustom(request, &response, mdb.TxOpt{})
		if err != nil {
			return errors.Annotatef(err, "%s target=%d request=%x", tag, speed, request.Bytes())
		}
		devConv.dev.Log.Debugf("%s target=%d request=%x response=%x", tag, speed, request.Bytes(), response.Bytes())
		return nil
	}}
}

func speedDistanceDuration(speedPerSecond float32, distance uint) time.Duration {
	return time.Duration(float32(distance)/speedPerSecond) * time.Second
}

func absDiffU16(a, b uint16) uint16 {
	if a >= b {
		return a - b
	}
	return b - a
}
