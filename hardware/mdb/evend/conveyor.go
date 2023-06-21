package evend

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
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
	speed      int8
	position   int16
}

func (c *DeviceConveyor) init(ctx context.Context) error {
	c.position = -1
	c.speed = -1
	g := state.GetGlobal(ctx)
	c.maxTimeout = ConveyorDefaultTimeout
	c.dev.DelayNext = 200 * time.Millisecond // empirically found lower total WaitReady
	c.Generic.Init(ctx, 0xd8, "conveyor", proto2)
	c.DoSetSpeed = c.newSetSpeed()
	g.Engine.Register(c.name+".set_speed(?)", c.DoSetSpeed)
	g.Engine.Register(c.name+".shake(?)",
		engine.FuncArg{Name: c.name + ".shake", F: func(ctx context.Context, arg engine.Arg) error {
			return g.Engine.Exec(ctx, c.Generic.WithRestart(c.shake(uint8(arg))))
		}})
	g.Engine.Register(c.name+".move(?)",
		engine.FuncArg{Name: c.name + ".m", F: func(ctx context.Context, arg engine.Arg) (err error) {
			for i := 1; i <= 2; i++ {
				if c.position == -1 {
					cs := c.speed
					if cs == -1 {
						c.setSpeed(1)
						cs = 1
					}
					c.moveWaitReadyWaitDone(int16(0)).Do(ctx)
					c.position = 0
					c.setSpeed(uint8(cs))
				}
				if err = c.moveNoWaitReadyNoWaitDone(int16(arg)).Do(ctx); err == nil {
					if err = c.moveWaitDone().Do(ctx); err == nil {
						c.position = int16(arg)
						if i > 1 {
							c.dev.TeleError(fmt.Errorf("restart fix problem"))
						}
						return
					}
				}
				err = fmt.Errorf("conveyor move %d=>%d error (%v) try(%d)", c.position, arg, err, i)
				c.dev.TeleError(err)
				c.position = -1
			}
			return err
		}})
	g.Engine.Register(c.name+".moveNoWait(?)",
		engine.FuncArg{Name: c.name + ".moveNoWait", F: func(ctx context.Context, arg engine.Arg) error {
			return c.moveNoWaitReadyNoWaitDone(int16(arg)).Do(ctx)
		}})
	g.Engine.Register(c.name+".movingDone", c.moveWaitDone())
	g.Engine.RegisterNewFunc(
		"conveyor.status",
		func(ctx context.Context) error {
			g.Log.Infof("%s.position:%d speed:%d", c.name, c.position, c.speed)
			return nil
		},
	)
	err := c.dev.Rst()
	return errors.Annotate(err, c.name+".init")
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

func (c *DeviceConveyor) shake(arg uint8) engine.Doer {
	tag := fmt.Sprintf("%s.shake:%d", c.name, arg)
	d := engine.NewSeq(tag).
		Append(c.Generic.NewWaitReady(tag)).
		Append(c.Generic.NewAction(tag, 0x03, byte(arg), 0)).
		Append(c.NewWaitDone(tag, c.maxTimeout))
	return d
}

func (c *DeviceConveyor) setSpeed(speed uint8) (err error) {
	c.speed = int8(speed)
	bs := []byte{c.dev.Address + 5, 0x10, speed}
	request := mdb.MustPacketFromBytes(bs, true)
	err = c.dev.Tx(request, nil)
	return
}

func (c *DeviceConveyor) newSetSpeed() engine.FuncArg {
	tag := c.name + ".set_speed"
	return engine.FuncArg{Name: tag, F: func(ctx context.Context, arg engine.Arg) error {
		return c.setSpeed(uint8(arg))
	}}
}
