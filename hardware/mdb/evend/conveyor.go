package evend

import (
	"context"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	// "github.com/juju/errors"
)

const (
	ConveyorDefaultTimeout      = 30 * time.Second
	ConveyorMinTimeout          = 1 * time.Second
	commandMove            byte = 1
	commandShake           byte = 2
	commandWaitAndShake    byte = 3
)

type DeviceConveyor struct { //nolint:maligned
	Generic

	DoSetSpeed  engine.FuncArg
	maxTimeout  time.Duration
	timeout     uint16
	speed       int8
	position    int16
	newPosition int16
}

func (c *DeviceConveyor) init(ctx context.Context) error {
	c.speed = -1
	c.timeout = 10 * 5
	g := state.GetGlobal(ctx)
	c.maxTimeout = ConveyorDefaultTimeout
	c.dev.DelayNext = 200 * time.Millisecond // empirically found lower total WaitReady
	c.Generic.Init(ctx, 0xd8, "conveyor", proto2)
	g.Engine.RegisterNewFuncAgr(c.name+".set_speed(?)", func(ctx context.Context, speed engine.Arg) error { return c.setSpeed(uint8(speed.(int16))) })
	g.Engine.RegisterNewFuncAgr("pc(?)", func(ctx context.Context, poolCount engine.Arg) error {
		return c.WaitSuccess(uint16(poolCount.(int16)), false)
	})

	g.Engine.RegisterNewFuncAgr(c.name+".moveNoWait(?)", func(ctx context.Context, position engine.Arg) error {
		return c.moveNoWait(position.(int16))
	})
	g.Engine.RegisterNewFunc(c.name+".movingDone", func(ctx context.Context) error { return c.movingDone() })
	g.Engine.RegisterNewFunc(c.name+".status", func(ctx context.Context) error {
		g.Log.Infof("%s.position:%d speed:%d", c.name, c.position, c.speed)
		return nil
	})
	g.Engine.RegisterNewFunc(c.name+".reset", func(ctx context.Context) error { return c.reset() })
	g.Engine.RegisterNewFuncAgr(c.name+".move(?)", func(ctx context.Context, position engine.Arg) error { return c.move(int16(position.(int16))) })
	g.Engine.RegisterNewFuncAgr(c.name+".shake(?)", func(ctx context.Context, cnt engine.Arg) error {
		return c.CommandWaitSuccess(uint16(cnt.(int16))*2*5, commandWaitAndShake, byte(cnt.(int16)), 0)
	})
	g.Engine.RegisterNewFuncAgr(c.name+".vibrate(?)", func(ctx context.Context, cnt engine.Arg) error {
		return c.CommandWaitSuccess(uint16(cnt.(int16))*2*5, commandShake, byte(cnt.(int16)), 0)
	})

	return c.reset()
}

func (c *DeviceConveyor) moveNoWait(position int16) error {
	c.newPosition = position
	c.position = -1
	return c.CommandNoWait(commandMove, byte(position&0xff), byte(position>>8))
}

func (c *DeviceConveyor) move(position int16) (err error) {
	if c.position == -1 {
		c.CommandWaitSuccess(c.timeout, commandMove, byte(0x00), byte(0x00))
		c.position = 0
	}
	if c.position == position {
		return nil
	}
	c.moveNoWait(position)
	return c.movingDone()
}

func (c *DeviceConveyor) movingDone() (err error) {
	if err = c.WaitSuccess(c.timeout, true); err != nil {
		return err
	}
	c.position = c.newPosition
	return nil
}

func (c *DeviceConveyor) reset() error {
	c.dev.SetupResponse = mdb.Packet{}
	c.position = -1
	return c.dev.Rst()
}

func (c *DeviceConveyor) setSpeed(speed uint8) (err error) {
	c.speed = int8(speed)
	bs := []byte{c.dev.Address + 5, 0x10, speed}
	request := mdb.MustPacketFromBytes(bs, true)
	err = c.dev.Tx(request, nil)
	return
}
