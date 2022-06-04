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

const DefaultShakeSpeed uint8 = 100

type DeviceMixer struct { //nolint:maligned
	Generic

	currentPos   int16 // estimated
	moveTimeout  time.Duration
	shakeTimeout time.Duration
	shakeSpeed   uint8
}

func (devMix *DeviceMixer) init(ctx context.Context) error {
	// devMix.currentPos = -1ec
	devMix.shakeSpeed = DefaultShakeSpeed
	g := state.GetGlobal(ctx)
	config := &g.Config.Hardware.Evend.Mixer
	keepaliveInterval := helpers.IntMillisecondDefault(config.KeepaliveMs, 0)
	devMix.moveTimeout = helpers.IntSecondDefault(config.MoveTimeoutSec, 10*time.Second)
	devMix.shakeTimeout = helpers.IntMillisecondDefault(config.ShakeTimeoutMs, 300*time.Millisecond)
	devMix.Generic.Init(ctx, 0xc8, "mixer", proto1)

	g.Engine.Register(devMix.name+".shake(?)",
		engine.FuncArg{Name: devMix.name + ".shake", F: func(ctx context.Context, arg engine.Arg) error {
			return g.Engine.Exec(ctx, devMix.Generic.WithRestart(devMix.shake(uint8(arg))))
		}})
	g.Engine.Register(devMix.name+".move(?)",
		engine.FuncArg{Name: devMix.name + ".move", F: func(ctx context.Context, arg engine.Arg) error {
			// return g.Engine.Exec(ctx, devMix.move(uint8(arg)))
			return g.Engine.Exec(ctx, devMix.Generic.WithRestart(devMix.move(uint8(arg))))
		}})
	g.Engine.Register(devMix.name+".fan_on", devMix.NewFan(true))
	g.Engine.Register(devMix.name+".fan_off", devMix.NewFan(false))
	g.Engine.Register(devMix.name+".shake_set_speed(?)",
		engine.FuncArg{Name: "evend.mixer.shake_set_speed", F: func(ctx context.Context, arg engine.Arg) error {
			devMix.shakeSpeed = uint8(arg)
			return nil
		}})

	g.Engine.RegisterNewFunc(
		"mixer.status",
		func(ctx context.Context) error {
			g.Log.Infof("%s.position:%d", devMix.name, devMix.currentPos)
			return nil
		},
	)

	err := devMix.Generic.FIXME_initIO(ctx)
	if keepaliveInterval > 0 {
		go devMix.Generic.dev.Keepalive(keepaliveInterval, g.Alive.StopChan())
	}
	return errors.Annotate(err, devMix.name+".init")
}

// 1step = 100ms
func (devMix *DeviceMixer) shake(steps uint8) engine.Doer {
	tag := fmt.Sprintf("%s.shake:%d,%d", devMix.name, steps, devMix.shakeSpeed)
	return engine.NewSeq(tag).
		Append(devMix.NewWaitReady(tag)).
		Append(devMix.Generic.NewAction(tag, 0x01, steps, devMix.shakeSpeed)).
		Append(devMix.NewWaitDone(tag, devMix.shakeTimeout*time.Duration(1+steps)))
}

func (devMix *DeviceMixer) NewFan(on bool) engine.Doer {
	tag := fmt.Sprintf("%s.fan:%t", devMix.name, on)
	arg := uint8(0)
	if on {
		arg = 1
	}
	return devMix.Generic.NewAction(tag, 0x02, arg, 0x00)
}

func (devMix *DeviceMixer) move(position uint8) engine.Doer {
	tag := fmt.Sprintf("%s.move:%d->%d", devMix.name, devMix.currentPos, position)
	devMix.currentPos = -1
	return engine.NewSeq(tag).
		Append(devMix.NewWaitReady(tag)).
		Append(devMix.Generic.NewAction(tag, 0x03, position, 0x64)).
		Append(devMix.NewWaitDone(tag, devMix.moveTimeout)).
		Append(engine.Func0{F: func() error { devMix.currentPos = int16(position); return nil }})
}
