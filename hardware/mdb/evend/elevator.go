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

type DeviceElevator struct { //nolint:maligned
	Generic

	moveTimeout time.Duration
	cPos        int8
	nPos        uint8
}

func (e *DeviceElevator) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	e.cPos = -1
	config := &g.Config.Hardware.Evend.Elevator
	keepaliveInterval := helpers.IntMillisecondDefault(config.KeepaliveMs, 0)
	e.moveTimeout = helpers.IntSecondDefault(config.MoveTimeoutSec, 10*time.Second)
	e.Generic.Init(ctx, 0xd0, "elevator", proto1)

	g.Engine.Register(e.name+".move(?)",
		engine.FuncArg{Name: e.name + ".move", F: func(ctx context.Context, arg engine.Arg) (err error) {
			if err = g.Engine.Exec(ctx, e.move(uint8(arg))); err == nil {
				e.cPos = int8(arg)
				return
			}
			e.dev.TeleError(err)
			e.dev.Reset()
			if err = g.Engine.Exec(ctx, e.move(uint8(arg))); err == nil {
				e.cPos = int8(arg)
				e.dev.TeleError(errors.Errorf("restart fix preview error"))
				return
			}
			e.dev.TeleError(errors.Annotatef(err, "two times error"))
			return
		}})

	g.Engine.Register(e.name+".moveNoWait(?)",
		engine.FuncArg{Name: e.name + ".moveNoWait", F: func(ctx context.Context, arg engine.Arg) error {
			return g.Engine.Exec(ctx, e.moveNoWait(uint8(arg)))
		}})

	g.Engine.RegisterNewFunc(
		"elevator.status",
		func(ctx context.Context) error {
			g.Log.Infof("%s.position:%d", e.name, e.cPos)
			return nil
		},
	)

	err := e.Generic.FIXME_initIO(ctx)
	if keepaliveInterval > 0 {
		go e.Generic.dev.Keepalive(keepaliveInterval, g.Alive.StopChan())
	}
	return errors.Annotate(err, e.name+".init")
}

func (e *DeviceElevator) move(position uint8) engine.Doer {
	tag := fmt.Sprintf("%s.move:%d->%d", e.name, e.cPos, position)
	d := engine.NewSeq(tag)
	if e.cPos == int8(position) {
		d.Append(engine.Func0{F: func() error { return nil }})
		return d
	}
	d.Append(e.NewWaitReady(tag))
	switch position {
	case 0, 100:
		d.Append(e.Generic.NewAction(tag, 0x03, position, 0x64))
	default:
		if e.cPos == -1 {
			d.Append(e.Generic.NewAction(tag, 0x03, 100, 0x64))
			d.Append(e.NewWaitDone(tag, e.moveTimeout))
			d.Append(e.NewWaitReady(tag))
		}
		e.cPos = -1
		d.Append(e.Generic.NewAction(tag, 0x03, position, 0x64))
	}

	d.Append(e.NewWaitDone(tag, e.moveTimeout))
	e.cPos = -1
	e.nPos = position
	return d
}

func (e *DeviceElevator) moveNoWait(position uint8) engine.Doer {
	tag := fmt.Sprintf("%s.move_no_wait:%d->%d", e.name, e.cPos, position)
	// c.cPos = -1
	return engine.NewSeq(tag).
		// Append(e.Generic.NewWaitReady(tag)).
		Append(e.Generic.NewAction(tag, 0x03, position, 0x64))
}
