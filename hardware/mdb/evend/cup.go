package evend

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
)

type DeviceCup struct {
	Generic
	timeout      uint16
	light        bool
	lightShedule struct {
		weekDay [7]worktime
	}
}

type worktime struct {
	BeginOfWork time.Duration
	EndOfWork   time.Duration
}

var Cup *DeviceCup

func (c *DeviceCup) init(ctx context.Context) error {
	c.Generic.Init(ctx, 0xe0, "cup", proto2)
	Cup = c
	g := state.GetGlobal(ctx)
	c.initLightSheduler(g.Config.UI_config.Front.LightShedule)
	c.timeout = uint16(g.Config.Hardware.Evend.Cup.TimeoutSec) * 5
	g.Engine.RegisterNewFunc(c.name+".ensure", func(ctx context.Context) error { return c.CommandWaitSuccess(c.timeout, 0x04) })
	g.Engine.RegisterNewFuncAgr(c.name+".dispense(?)", func(ctx context.Context, _ engine.Arg) error { return c.CommandWaitSuccess(c.timeout, 0x01) })
	g.Engine.RegisterNewFunc(c.name+".wait_complete", func(ctx context.Context) error { return c.WaitSuccess(c.timeout, true) })
	g.Engine.RegisterNewFunc(c.name+".light_on", func(ctx context.Context) error { return c.LightOn() })
	g.Engine.RegisterNewFunc(c.name+".light_off", func(ctx context.Context) error { return c.LightOff() })
	g.Engine.RegisterNewFunc(c.name+".reset", func(ctx context.Context) error { return c.dev.Rst() })
	g.Engine.RegisterNewFunc(c.name+".light_on_schedule", func(ctx context.Context) error {
		if !c.lightShouldWork() {
			if c.light {
				return c.LightOff()
			}
			return nil
		}
		return c.LightOn()
	})
	if err := c.dev.Rst(); err != nil {
		return fmt.Errorf("%s init error:%v", c.name, err)
	}
	return nil
}

func (c *DeviceCup) Reset() error {
	return c.dev.Rst()
}

func (c *DeviceCup) LightOn() error {
	if c.light {
		return nil
	}
	c.dev.Log.Info("light on")
	c.Command(0x02)
	c.light = true
	return c.WaitSuccess(c.timeout, false)
}

func (c *DeviceCup) LightOff() error {
	if !c.light {
		return nil
	}
	c.dev.Log.Info("light off")
	c.light = false
	c.Command(0x03)
	return c.WaitSuccess(c.timeout, false)
}

// sheduler front light

// if BeginOfWork & EndOfWork = 0 or empty then light allway on
// if BeginOfWork = EndOfWork and <> 0 then light allway off
// example (1-5 8:00-20:00) (6 10:00-13:00) (0 11:00-12:00) or (* 11:00-18:21)
// parts format
// 1 - week number *=all days 0=saturday 1=monday
// 2 - range days
// 3,4 - begin hours, minutes
// 5,6 - end hours, minutes
func (c *DeviceCup) initLightSheduler(sh string) {
	wd := `([0-6]|\*)[-]?([0-6])? ([01]?[0-9]|2[0-3]):([0-5][0-9])-([01]?[0-9]|2[0-3]):([0-5][0-9])`
	cmd := regexp.MustCompile(wd)

	parts := cmd.FindAllStringSubmatch(sh, 7)
	if (sh != "") && (len(parts) == 0) {
		c.dev.Log.WarningF("light shedule string error: %s", sh)
		return
	}
	for _, v := range parts {
		c.dev.Log.Infof("add light shedule %v", v[0])
		switch v[1] {
		case "*":
			for i := 0; i < 7; i++ {
				c.writeShedule(i, v)
			}
		default:
			w, _ := strconv.Atoi(v[1])
			if v[2] == "" {
				c.writeShedule(w, v)
			} else {
				e, _ := strconv.Atoi(v[2])
				for i := w; i <= e; i++ {
					c.writeShedule(i, v)
				}
			}
		}
	}
}

func (s *DeviceCup) writeShedule(w int, v []string) {
	s.lightShedule.weekDay[w].BeginOfWork = textTimeToDudation(v[3], v[4])
	s.lightShedule.weekDay[w].EndOfWork = textTimeToDudation(v[5], v[6])
}

func textTimeToDudation(hours string, minutes string) time.Duration {
	h, _ := strconv.Atoi(hours)
	m, _ := strconv.Atoi(minutes)
	return time.Hour*time.Duration(h) + time.Minute*time.Duration(m)
}

func (s *DeviceCup) lightShouldWork() bool {
	t := time.Now()
	w := t.Weekday()
	if s.lightShedule.weekDay[w].BeginOfWork == s.lightShedule.weekDay[w].EndOfWork {
		return s.lightShedule.weekDay[w].BeginOfWork == 0
	}
	_, dif := t.Zone()
	startOfDay := t.Truncate(24 * time.Hour).Add(time.Second * time.Duration(-dif))
	startWorkinHours := startOfDay.Add(s.lightShedule.weekDay[w].BeginOfWork)
	endWorkinHours := startOfDay.Add(s.lightShedule.weekDay[w].EndOfWork)
	return t.After(startWorkinHours) && t.Before(endWorkinHours)
}
