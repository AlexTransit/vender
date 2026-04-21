package evend

import (
	"context"
	"fmt"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

// error codes:
// 35	DEVICE_ERROR_REVERSE_DISCONNECTED
// 36	DEVICE_ERROR_REVERSE_HIGH_LOAD
// 37	DEVICE_ERROR_REVERSE_TOP_SENSOR
// 38	DEVICE_ERROR_REVERSE_BOTTOM_SENSOR
// 39	DEVICE_ERROR_REVERSE_NOT_IN_TOP_POSITION

// текущая проша миксера подьемника, требует определенных действий. надо обязательно "проехать" с нуля на 100
// тогда можно, один раз!, установить промежуточную позицию.
//  иначе установка "промежуточной" позиции сделает движеие к 100
// вплоть до ошибки по перегрузке.
// прошивка сначала посылает команду на исполнение, а потом опрашивает сенсоры.
// находясь в нулевой позиции, по команде позиция=0, прошивка дает питание на мотор что может вызвать ошибку 39
// порядок дивжений едеватора только такой.
// позиция 0 - позиция 100 - промежуточная позиция- позиция 0. ( иначе все станет раком)

type MiherElevator struct {
	Generic
	currentPos int8
	newPos     int8
	shakeSpeed uint8
}

func (me *MiherElevator) InitMiherElevator(ctx context.Context, address uint8, name string, proto evendProtocol) error {
	me.Generic.Init(ctx, address, name, proto)
	g := state.GetGlobal(ctx)
	me.currentPos = -1
	g.Engine.RegisterNewFunc(me.name+".reset", func(ctx context.Context) error { return me.reset() })
	g.Engine.RegisterNewFuncAgr(me.name+".moveNoWait(?)", func(ctx context.Context, arg engine.Arg) error { return me.moveNoWait(int8(arg.(int16))) })
	g.Engine.RegisterNewFuncAgr(me.name+".WaitSuccess(?)", func(ctx context.Context, arg engine.Arg) error { return me.WaitSuccess(uint16(arg.(int16)*5+5), true) })
	g.Engine.RegisterNewFunc(me.name+".movingComplete", func(ctx context.Context) error { return me.mvComplete() })
	g.Engine.RegisterNewFuncAgr(me.name+".move(?)", func(ctx context.Context, arg engine.Arg) error { return me.move(int8(arg.(int16))) })

	g.Engine.RegisterNewFunc(me.name+".status", func(ctx context.Context) error {
		g.Log.Infof("%s.position:%d shake speed:%d", me.name, me.currentPos, me.shakeSpeed)
		return nil
	})

	return me.dev.Rst()
}

func (me *MiherElevator) move(position int8) error {
	me.dev.Action = fmt.Sprintf("%s move %d=>%d", me.name, me.currentPos, position)
	if err := me.moveNoWait(position); err != nil {
		return fmt.Errorf("send command(%v) error(%v)", me.dev.Action, err)
	}
	return me.mvComplete()
}

func (me *MiherElevator) moveNoWait(position int8) error {
	me.currentPos = -1
	me.newPos = position
	return me.Command(0x03, byte(position), 0x64)
}

func (me *MiherElevator) mvComplete() error {
	err := me.WaitSuccess(100, true) // FIXME timeout to config
	if err == nil {
		me.currentPos = me.newPos
		me.dev.Action = ""
	}
	return err
}

func (me *MiherElevator) reset() error {
	me.currentPos = -1
	return me.dev.Rst()
}
