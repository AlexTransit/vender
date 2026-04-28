package evend

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

// error codes:
// 31	DEVICE_ERROR_MOTOR_DISCONNECTED
// 32	DEVICE_ERROR_MOTOR_HIGH_LOAD
// 33	DEVICE_ERROR_SHAKER_DISCONNECTED
// 34	DEVICE_ERROR_SHAKER_HIGH_LOAD

const DefaultShakeSpeed uint8 = 100

type DeviceMixer struct { //nolint:maligned
	MiherElevator

	// cPos       int8
	// nPos       int8
	// shakeSpeed uint8
}

func (m *DeviceMixer) init(ctx context.Context) error {
	m.currentPos = -1
	m.shakeSpeed = DefaultShakeSpeed
	g := state.GetGlobal(ctx)
	err := m.InitMiherElevator(ctx, 0xc8, "mixer", proto1)
	g.Engine.Register(m.name+".shake(?)",
		engine.FuncArg{Name: m.name + ".shake", F: func(ctx context.Context, arg engine.Arg) (err error) {
			m.dev.Action = fmt.Sprintf("miher shake(%d)", arg)
			return m.shake(uint8(arg.(int16)))
		}})
	g.Engine.RegisterNewFuncAgr(m.name+".shakeNoWait(?)", func(ctx context.Context, arg engine.Arg) error { return m.shakeNoWait(uint8(arg.(int16))) })
	g.Engine.Register(m.name+".fan_on", m.NewFan(true))
	g.Engine.Register(m.name+".fan_off", m.NewFan(false))
	g.Engine.Register(m.name+".shake_set_speed(?)",
		engine.FuncArg{Name: "evend.mixer.shake_set_speed", F: func(ctx context.Context, arg engine.Arg) error {
			m.shakeSpeed = uint8(arg.(int16))
			return nil
		}})
	return err
}

func (m *DeviceMixer) shakeNoWait(steps uint8) (err error) {
	if err = m.Command(0x01, byte(steps), m.shakeSpeed); err != nil {
		return err
	}
	return m.WaitSuccess(1, false)
}

// 1step = 100ms
func (m *DeviceMixer) shake(steps uint8) (err error) {
	if err = m.shakeNoWait(steps); err != nil {
		return
	}
	if steps > 4 {
		time.Sleep(time.Duration(steps-4) * 100 * time.Millisecond)
	}
	return m.WaitSuccess(20, false)
}

// --------------------------------------------------------

func (m *DeviceMixer) NewFan(on bool) engine.Doer {
	tag := fmt.Sprintf("%s.fan:%t", m.name, on)
	arg := uint8(0)
	if on {
		arg = 1
	}
	return m.NewAction(tag, 0x02, arg, 0x00)
}
