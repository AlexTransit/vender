package evend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

type DeviceElevator struct { //nolint:maligned
	Generic

	cPos        int8
	nPos        uint8
	moveTimeout time.Duration
}

// текущая проша подьемника, требует определенных действий. надо обязательно "проехать" с нуля на 100
// тогда можно, один раз, установить промежуточную позицию иначе установка "промежуточной" позиции сделает движеие к 100
// вплоть до ошибки по перегрузке.
// прошивка сначала посылает команду на исполнение, а потом опрашивает сенсоры.
// находясь в нулевой позиции, по команде позиция=0, прошивка дает питание на мотор что может вызвать ошибку 39 (высокая нагрузка)
// порядок дивжений едеватора только такой.
// позиция 0 - позиция 100 - промежуточная позиция- позиция 0. ( иначе все станет раком)

func (e *DeviceElevator) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	config := &g.Config.Hardware.Evend.Elevator
	e.moveTimeout = helpers.IntSecondDefault(config.MoveTimeoutSec, time.Second)
	e.Generic.Init(ctx, 0xd0, "elevator", proto1)

	g.Engine.RegisterNewFunc(e.name+".reset", func(ctx context.Context) error { return e.reset() })
	g.Engine.RegisterNewFuncAgr(e.name+".moveNoWait(?)", func(ctx context.Context, arg engine.Arg) error { return e.moveNoWait(uint8(arg.(int16))) })
	g.Engine.RegisterNewFunc(e.name+".moveComplete", func(ctx context.Context) error { return e.mvComplete() })
	g.Engine.Register(e.name+".move(?)", engine.FuncArg{Name: e.name + ".move", F: func(ctx context.Context, arg engine.Arg) (err error) {
		if arg == e.cPos {
			return nil
		}
		previewPosition := e.cPos
		for i := 1; i <= 2; i++ {
			er := e.move(uint8(arg.(int16)))
			if er == nil {
				if i > 1 {
					e.dev.TeleError(fmt.Errorf("restart fix error (%v)", err))
				}
				return nil
			}
			err = errors.Join(err, er)
			// FIXME тут можно добавть скрипт действий после ошибки
			if e.dev.ErrorCode() == 36 { // reverse high load
				if !((previewPosition == 100 && arg == 0) || (previewPosition == 0 && arg == 100)) {
					e.reset()
					break
				}
			}
			e.reset()
			time.Sleep(5 * time.Second)
		}
		return err
	}})

	g.Engine.RegisterNewFunc(
		"elevator.status",
		func(ctx context.Context) error {
			g.Log.Infof("%s.position:%d", e.name, e.cPos)
			return nil
		},
	)
	return e.reset()
}

func (e *DeviceElevator) reset() error {
	e.cPos = -1
	return e.dev.Rst()
}

func (e *DeviceElevator) moveNoWait(position uint8) (err error) {
	e.cPos = -1
	e.nPos = position
	// return m.Command([]byte{0x03, byte(position), 0x64})
	return e.Command(0x03, byte(position), 0x64)
}

func (e *DeviceElevator) move(position uint8) (err error) {
	e.dev.Action = fmt.Sprintf("%s move %d=>%d", e.name, e.cPos, position)
	if err = e.moveNoWait(position); err != nil {
		// e.errorCode =
		return fmt.Errorf("send command(%v) error(%v)", e.dev.Action, err)
	}
	return e.mvComplete()
}

func (e *DeviceElevator) mvComplete() (err error) {
	err = e.WaitSuccess(100, true) // FIXME timeout to config
	if err == nil {
		e.cPos = int8(e.nPos)
		e.dev.Action = ""
	}
	return err
}
