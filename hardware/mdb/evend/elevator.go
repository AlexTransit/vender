package evend

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/juju/errors"
)

type DeviceElevator struct { //nolint:maligned
	Generic

	// earlyPos    int16 // estimated
	// currentPos  int16 // estimated
	moveTimeout time.Duration
}

func (devElv *DeviceElevator) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	config := &g.Config.Hardware.Evend.Elevator
	keepaliveInterval := helpers.IntMillisecondDefault(config.KeepaliveMs, 0)
	devElv.moveTimeout = helpers.IntSecondDefault(config.MoveTimeoutSec, 10*time.Second)
	devElv.Generic.Init(ctx, 0xd0, "elevator", proto1)

	g.Engine.Register(devElv.name+".move(?)",
		engine.FuncArg{Name: devElv.name + ".move", F: func(ctx context.Context, arg engine.Arg) error {
			return g.Engine.Exec(ctx, devElv.move(uint8(arg)))
		}})

	// g.Engine.RegisterNewFunc(
	// 	"elevator.status",
	// 	func(ctx context.Context) error {
	// 		g.Log.Infof("position:%d", global.GBL.HW.Elevator)
	// 		return nil
	// 	},
	// )

	err := devElv.Generic.FIXME_initIO(ctx)
	if keepaliveInterval > 0 {
		go devElv.Generic.dev.Keepalive(keepaliveInterval, g.Alive.StopChan())
	}
	return errors.Annotate(err, devElv.name+".init")
}

func (devElv *DeviceElevator) move(position uint8) engine.Doer {
	// cp := global.GetEnv(devElv.name + ".position")
	cp := types.VMC.HW.Elevator.Position
	types.VMC.HW.Elevator.Position = 255
	mp := fmt.Sprintf("%d->%d", cp, position)
	types.Log.Infof(devElv.name+".position = %s", mp)
	tag := fmt.Sprintf("%s.move:%s", devElv.name, mp)
	return engine.NewSeq(tag).
		Append(devElv.NewWaitReady(tag)).
		Append(devElv.Generic.NewAction(tag, 0x03, position, 0x64)).
		Append(devElv.NewWaitDone(tag, devElv.moveTimeout)).
		Append(engine.Func0{F: func() error { types.VMC.HW.Elevator.Position = position; return nil }})
}
