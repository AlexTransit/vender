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
	cPos       int16
	nPos       int16
}

func (c *DeviceConveyor) init(ctx context.Context) error {
	c.cPos = -1
	g := state.GetGlobal(ctx)
	devConfig := &g.Config.Hardware.Evend.Conveyor
	keepaliveInterval := helpers.IntMillisecondDefault(devConfig.KeepaliveMs, 0)
	c.maxTimeout = ConveyorDefaultTimeout
	c.minSpeed = uint16(devConfig.MinSpeed)
	if c.minSpeed == 0 {
		c.minSpeed = 200
	}
	g.Log.Debugf("evend.conveyor minSpeed=%d maxTimeout=%v keepalive=%v", c.minSpeed, c.maxTimeout, keepaliveInterval)
	// c.dev.DelayNext = 245 * time.Millisecond // empirically found lower total WaitReady
	c.dev.DelayNext = 200 * time.Millisecond // empirically found lower total WaitReady
	c.Generic.Init(ctx, 0xd8, "conveyor", proto2)
	c.DoSetSpeed = c.newSetSpeed()
	g.Engine.Register(c.name+".set_speed(?)", c.DoSetSpeed)

	g.Engine.Register(c.name+".shake(?)",
		engine.FuncArg{Name: c.name + ".shake", F: func(ctx context.Context, arg engine.Arg) error {
			return g.Engine.Exec(ctx, c.Generic.WithRestart(c.shake(uint8(arg))))
		}})

	g.Engine.Register(c.name+".move(?)",
		engine.FuncArg{Name: c.name + ".move", F: func(ctx context.Context, arg engine.Arg) (err error) {
			if err = g.Engine.Exec(ctx, c.move(int16(arg))); err == nil {
				c.cPos = int16(arg)
				return
			}
			c.dev.TeleError(err)
			c.dev.Reset()
			// AlexM ToDo тут нужно добавить скрипт действий в случае ошибки
			if err = g.Engine.Exec(ctx, c.move(int16(arg))); err == nil {
				c.cPos = int16(arg)
				c.dev.TeleError(errors.Errorf("restart fix preview error"))
				return
			}
			c.dev.TeleError(errors.Annotatef(err, "two times error"))
			return
		}})

	g.Engine.Register(c.name+".moveNoWait(?)",
		engine.FuncArg{Name: c.name + ".moveNoWait", F: func(ctx context.Context, arg engine.Arg) error {
			return g.Engine.Exec(ctx, c.moveNoWait(int16(arg)))
		}})

	g.Engine.RegisterNewFunc(
		"conveyor.status",
		func(ctx context.Context) error {
			g.Log.Infof("%s.position:%d", c.name, c.cPos)
			return nil
		},
	)

	err := c.dev.Rst()
	if keepaliveInterval > 0 {
		go c.Generic.dev.Keepalive(keepaliveInterval, g.Alive.StopChan())
	}
	return errors.Annotate(err, c.name+".init")
}

func (c *DeviceConveyor) move(position int16) engine.Doer {
	tag := fmt.Sprintf("%s.move:%d->%d", c.name, c.cPos, position)
	d := engine.NewSeq(tag)
	if c.cPos == position && position != 0 {
		d.Append(engine.Func0{F: func() error { return nil }})
		return d
	}
	d.Append(c.Generic.NewWaitReady(tag))
	if c.cPos == -1 && position != 0 {
		d.Append(c.Generic.NewAction(tag, 0x01, byte(0), byte(0)))
		d.Append(c.NewWaitDone(tag, c.maxTimeout))
	}
	c.cPos = -1
	c.nPos = position
	d.Append(c.Generic.NewAction(tag, 0x01, byte(position&0xff), byte(position>>8))).
		Append(c.NewWaitDone(tag, c.maxTimeout))
	return d
}

func (c *DeviceConveyor) moveNoWait(position int16) engine.Doer {
	tag := fmt.Sprintf("%s.move_no_wait:%d->%d", c.name, c.cPos, position)
	// c.cPos = -1
	return engine.NewSeq(tag).
		Append(c.Generic.NewWaitReady(tag)).
		Append(c.Generic.NewAction(tag, 0x01, byte(position&0xff), byte(position>>8)))
}

func (c *DeviceConveyor) shake(arg uint8) engine.Doer {
	tag := fmt.Sprintf("%s.shake:%d", c.name, arg)
	d := engine.NewSeq(tag).
		Append(c.Generic.NewWaitReady(tag)).
		Append(c.Generic.NewAction(tag, 0x03, byte(arg), 0)).
		Append(c.NewWaitDone(tag, c.maxTimeout))
	return d
}

func (c *DeviceConveyor) newSetSpeed() engine.FuncArg {
	tag := c.name + ".set_speed"

	return engine.FuncArg{Name: tag, F: func(ctx context.Context, arg engine.Arg) error {
		speed := uint8(arg)
		bs := []byte{c.dev.Address + 5, 0x10, speed}
		request := mdb.MustPacketFromBytes(bs, true)
		response := mdb.Packet{}
		err := c.dev.TxCustom(request, &response, mdb.TxOpt{})
		if err != nil {
			return errors.Annotatef(err, "%s target=%d request=%x", tag, speed, request.Bytes())
		}
		c.dev.Log.Debugf("%s target=%d request=%x response=%x", tag, speed, request.Bytes(), response.Bytes())
		return nil
	}}
}
