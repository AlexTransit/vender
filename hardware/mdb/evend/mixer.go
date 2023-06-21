package evend

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

const DefaultShakeSpeed uint8 = 100

type DeviceMixer struct { //nolint:maligned
	Generic

	cPos         int8
	nPos         int8
	moveTimeout  time.Duration
	shakeTimeout time.Duration
	shakeSpeed   uint8
}

func (m *DeviceMixer) init(ctx context.Context) error {
	m.cPos = -1
	m.shakeSpeed = DefaultShakeSpeed
	g := state.GetGlobal(ctx)
	config := &g.Config.Hardware.Evend.Mixer
	m.moveTimeout = helpers.IntSecondDefault(config.MoveTimeoutSec, 10*time.Second)
	m.shakeTimeout = helpers.IntMillisecondDefault(config.ShakeTimeoutMs, 300*time.Millisecond)
	m.Generic.Init(ctx, 0xc8, "mixer", proto1)

	g.Engine.Register(m.name+".shake(?)",
		engine.FuncArg{Name: m.name + ".shake", F: func(ctx context.Context, arg engine.Arg) (err error) {
			// return g.Engine.Exec(ctx, m.Generic.WithRestart(m.shake(uint8(arg))))
			err = m.shake(uint8(arg))
			return err
		}})

	g.Engine.Register(m.name+".moveNoWait(?)",
		engine.FuncArg{Name: m.name + ".moveNoWait", F: func(ctx context.Context, arg engine.Arg) error {
			return m.mv(int8(arg))
		}})

	g.Engine.RegisterNewFunc(m.name+".movingComplete", func(ctx context.Context) error { return m.mvComplete() })

	g.Engine.Register(m.name+".move(?)", engine.FuncArg{Name: m.name + ".move", F: func(ctx context.Context, arg engine.Arg) (err error) {
		for i := 1; i <= 2; i++ {
			if err = m.move(int8(arg)); err == nil {
				if i > 1 {
					m.dev.TeleError(fmt.Errorf("restart fix error"))
				}
				return
			}
			m.cPos = -1
			// FIXME тут можно добавть скрипт действий после ошибки
			m.dev.Rst()
			time.Sleep(5 * time.Second)
		}
		return err
	}})
	g.Engine.Register(m.name+".fan_on", m.NewFan(true))
	g.Engine.Register(m.name+".fan_off", m.NewFan(false))
	g.Engine.Register(m.name+".shake_set_speed(?)",
		engine.FuncArg{Name: "evend.mixer.shake_set_speed", F: func(ctx context.Context, arg engine.Arg) error {
			m.shakeSpeed = uint8(arg)
			return nil
		}})

	g.Engine.RegisterNewFunc("mixer.status", func(ctx context.Context) error {
		g.Log.Infof("%s.position:%d shake speed:%d", m.name, m.cPos, m.shakeSpeed)
		return nil
	})
	err := m.dev.Rst()
	return err
}

func (m *DeviceMixer) move(position int8) (err error) {
	m.dev.Action = fmt.Sprintf("mixer move %d=>%d", m.cPos, position)
	if err = m.mv(position); err != nil {
		return fmt.Errorf("send command(%v) error(%v)", m.dev.Action, err)
	}
	return m.mvComplete()
}

func (m *DeviceMixer) mvComplete() (err error) {
	err = m.Proto1PollWaitSuccess(100) // FIXME timeout to config
	if err == nil {
		m.cPos = m.nPos
		m.dev.Action = ""
	}
	return err
}

func (m *DeviceMixer) mv(position int8) (err error) {
	m.cPos = -1
	m.nPos = position
	return m.Command([]byte{0x03, byte(position), 0x64})
}

func (m *DeviceMixer) sh(steps uint8) (err error) {
	if err = m.Command([]byte{0x01, byte(steps), m.shakeSpeed}); err != nil {
		return err
	}
	return m.Proto1PollWaitSuccess(1)
}

// 1step = 100ms
func (m *DeviceMixer) shake(steps uint8) (err error) {
	if err = m.sh(steps); err != nil {
		return
	}
	if err = m.Proto1PollWaitSuccess(1); err != nil {
		return err
	}
	time.Sleep(time.Duration(steps-4) * 100 * time.Millisecond)
	return m.Proto1PollWaitSuccess(1)
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
