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

	cPos         int8 // estimated
	moveTimeout  time.Duration
	shakeTimeout time.Duration
	shakeSpeed   uint8
}

func (m *DeviceMixer) init(ctx context.Context) error {
	m.cPos = -1
	m.shakeSpeed = DefaultShakeSpeed
	g := state.GetGlobal(ctx)
	config := &g.Config.Hardware.Evend.Mixer
	keepaliveInterval := helpers.IntMillisecondDefault(config.KeepaliveMs, 0)
	m.moveTimeout = helpers.IntSecondDefault(config.MoveTimeoutSec, 10*time.Second)
	m.shakeTimeout = helpers.IntMillisecondDefault(config.ShakeTimeoutMs, 300*time.Millisecond)
	m.Generic.Init(ctx, 0xc8, "mixer", proto1)

	g.Engine.Register(m.name+".shake(?)",
		engine.FuncArg{Name: m.name + ".shake", F: func(ctx context.Context, arg engine.Arg) error {
			return g.Engine.Exec(ctx, m.Generic.WithRestart(m.shake(uint8(arg))))
		}})
	g.Engine.Register(m.name+".move(?)",
		engine.FuncArg{Name: m.name + ".move", F: func(ctx context.Context, arg engine.Arg) error {
			// return g.Engine.Exec(ctx, m.move(uint8(arg)))
			return g.Engine.Exec(ctx, m.Generic.WithRestart(m.move(uint8(arg))))
		}})
	g.Engine.Register(m.name+".fan_on", m.NewFan(true))
	g.Engine.Register(m.name+".fan_off", m.NewFan(false))
	g.Engine.Register(m.name+".shake_set_speed(?)",
		engine.FuncArg{Name: "evend.mixer.shake_set_speed", F: func(ctx context.Context, arg engine.Arg) error {
			m.shakeSpeed = uint8(arg)
			return nil
		}})

	g.Engine.RegisterNewFunc(
		"mixer.status",
		func(ctx context.Context) error {
			g.Log.Infof("%s.position:%d", m.name, m.cPos)
			return nil
		},
	)

	err := m.Generic.FIXME_initIO(ctx)
	if keepaliveInterval > 0 {
		go m.Generic.dev.Keepalive(keepaliveInterval, g.Alive.StopChan())
	}
	return errors.Annotate(err, m.name+".init")
}

// 1step = 100ms
func (m *DeviceMixer) shake(steps uint8) engine.Doer {
	tag := fmt.Sprintf("%s.shake:%d,%d", m.name, steps, m.shakeSpeed)
	return engine.NewSeq(tag).
		Append(m.NewWaitReady(tag)).
		Append(m.Generic.NewAction(tag, 0x01, steps, m.shakeSpeed)).
		Append(m.NewWaitDone(tag, m.shakeTimeout*time.Duration(1+steps)))
}

func (m *DeviceMixer) NewFan(on bool) engine.Doer {
	tag := fmt.Sprintf("%s.fan:%t", m.name, on)
	arg := uint8(0)
	if on {
		arg = 1
	}
	return m.Generic.NewAction(tag, 0x02, arg, 0x00)
}

func (m *DeviceMixer) move(position uint8) engine.Doer {
	tag := fmt.Sprintf("%s.move:%d->%d", m.name, m.cPos, position)
	d := engine.NewSeq(tag)
	if m.cPos == int8(position) {
		d.Append(engine.Func0{F: func() error { return nil }})
		return d
	}
	d.Append(m.NewWaitReady(tag))
	switch position {
	case 0, 100:
		d.Append(m.Generic.NewAction(tag, 0x03, position, 0x64))
	default:
		if m.cPos == -1 {
			d.Append(m.Generic.NewAction(tag, 0x03, 0, 0x64))
			d.Append(m.NewWaitDone(tag, m.moveTimeout))
			d.Append(m.NewWaitReady(tag))
		}
		m.cPos = -1
		d.Append(m.Generic.NewAction(tag, 0x03, position, 0x64))
	}

	d.Append(m.NewWaitDone(tag, m.moveTimeout))
	d.Append(engine.Func0{F: func() error { m.cPos = int8(position); return nil }})
	return d
}
