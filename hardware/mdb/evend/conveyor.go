package evend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	// "github.com/juju/errors"
)

const ConveyorDefaultTimeout = 30 * time.Second
const ConveyorMinTimeout = 1 * time.Second
const commandMove byte = 1
const commandShake byte = 2
const commandWaitAndShake byte = 3

type DeviceConveyor struct { //nolint:maligned
	Generic

	DoSetSpeed engine.FuncArg
	maxTimeout time.Duration
	timeout    uint16
	speed      int8
	position   int16
}

func (c *DeviceConveyor) init(ctx context.Context) error {
	c.position = -1
	c.speed = -1
	c.timeout = 10 * 5
	g := state.GetGlobal(ctx)
	c.maxTimeout = ConveyorDefaultTimeout
	c.dev.DelayNext = 200 * time.Millisecond // empirically found lower total WaitReady
	c.Generic.Init(ctx, 0xd8, "conveyor", proto2)
	g.Engine.RegisterNewFuncAgr(c.name+".set_speed(?)", func(ctx context.Context, speed engine.Arg) error { return c.setSpeed(uint8(speed)) })
	g.Engine.Register(c.name+".moveNoWait(?)",
		engine.FuncArg{Name: c.name + ".moveNoWait", F: func(ctx context.Context, arg engine.Arg) error {
			return c.moveNoWaitReadyNoWaitDone(int16(arg)).Do(ctx)
		}})
	g.Engine.Register(c.name+".movingDone", c.moveWaitDone())
	g.Engine.RegisterNewFunc("conveyor.status", func(ctx context.Context) error {
		g.Log.Infof("%s.position:%d speed:%d", c.name, c.position, c.speed)
		return nil
	})
	g.Engine.RegisterNewFunc(c.name+".reset", func(ctx context.Context) error { return c.reset() })
	g.Engine.RegisterNewFuncAgr(c.name+".move(?)", func(ctx context.Context, arg engine.Arg) error { return c.move(int16(arg)) })
	g.Engine.RegisterNewFuncAgr(c.name+".shake(?)", func(ctx context.Context, arg engine.Arg) error { return c.shake(int8(arg)) })
	g.Engine.RegisterNewFuncAgr(c.name+".vibrate(?)", func(ctx context.Context, arg engine.Arg) error { return c.vibrate(int8(arg)) })

	err := c.reset()
	return errors.Join(fmt.Errorf(c.name+".init"), err)
}

func (c *DeviceConveyor) shake(cnt int8) (err error) {
	if err = c.CommandWaitSuccess(uint16(cnt)*2*5, commandWaitAndShake, byte(cnt), 0); err == nil {
		return c.ReadError()
	}
	return
}

func (c *DeviceConveyor) vibrate(cnt int8) (err error) {
	if err = c.CommandWaitSuccess(uint16(cnt)*2*5, commandShake, byte(cnt), 0); err == nil {
		return c.ReadError()
	}
	return
}

func (c *DeviceConveyor) move(position int16) (err error) {
	if err = c.mv(position); err == nil {
		return
	}
	c.dev.TeleError(err)
	c.reset()
	time.Sleep(5000 * time.Millisecond)
	if err = c.mv(position); err == nil {
		return
	}
	return err
}

func (c *DeviceConveyor) mv(position int16) (err error) {
	c.dev.Action = fmt.Sprintf("%s move %v=>%v ", c.name, c.position, position)
	defer func() {
		if err != nil {
			err = errors.Join(fmt.Errorf(c.dev.Action), err)
		}
	}()
	if c.position == -1 {
		if err = c.setZero(); err != nil {
			err = errors.Join(fmt.Errorf("%s move to zero error", c.name), err)
			return err
		}
	}
	if position == c.position {
		return nil
	}
	if err = c.CommandWaitSuccess(c.timeout, commandMove, byte(position&0xff), byte(position>>8)); err == nil {
		if err = c.ReadError(); err != nil {
			err = errors.Join(fmt.Errorf(c.dev.Action), err)
			c.position = -1
			return err
		}
		c.position = position
		return nil
	}
	return err
}

func (c *DeviceConveyor) setZero() (err error) {
	if err = c.CommandWaitSuccess(c.timeout, commandMove, 0, 0); err != nil {
		return
	}
	if errb := c.ReadError_proto2(); errb != 0 {
		return fmt.Errorf("device:%v error:%v", c.dev.Name(), errb)
	}
	c.position = 0
	return nil
}

func (c *DeviceConveyor) reset() error {
	c.dev.SetupResponse = mdb.Packet{}
	c.position = -1
	return c.dev.Rst()
}

func (c *DeviceConveyor) moveWaitReadyWaitDone(position int16) engine.Doer {
	tag := fmt.Sprintf("%s.move:%d->%d", c.name, c.position, position)
	return engine.NewSeq(tag).
		Append(c.NewWaitReady(tag)).
		Append(c.NewAction(tag, 0x01, byte(position&0xff), byte(position>>8))).
		Append(c.NewWaitDone(tag, c.maxTimeout))
}

func (c *DeviceConveyor) moveWaitDone() engine.Doer {
	return engine.NewSeq("").
		Append(c.NewWaitReady("moving")).
		Append(c.NewWaitDone("moiving", c.maxTimeout))
}

func (c *DeviceConveyor) moveNoWaitReadyNoWaitDone(position int16) engine.Doer {
	return engine.NewSeq("").Append(c.Generic.NewAction("send move command", 0x01, byte(position&0xff), byte(position>>8)))
}

func (c *DeviceConveyor) setSpeed(speed uint8) (err error) {
	c.speed = int8(speed)
	bs := []byte{c.dev.Address + 5, 0x10, speed}
	request := mdb.MustPacketFromBytes(bs, true)
	err = c.dev.Tx(request, nil)
	return
}
