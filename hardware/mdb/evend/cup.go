package evend

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
)

const DefaultTimeout = 30

type DeviceCup struct {
	Generic
	timeout      uint8
	Light        bool
	lightShedule struct {
		weekDay [7]worktime
	}
}

//	type lightShedule struct {
//		weekDay [7]worktime
//	}
type worktime struct {
	BeginOfWork time.Duration
	EndOfWork   time.Duration
}

func (c *DeviceCup) init(ctx context.Context) error {
	c.Generic.Init(ctx, 0xe0, "cup", proto2)

	g := state.GetGlobal(ctx)
	c.initLightSheduler(g.Config.UI.Front.LightShedule)
	c.timeout = uint8(helpers.IntConfigDefault(g.Config.Hardware.Evend.Cup.DispenseTimeoutSec, 10)) * 5
	g.Engine.RegisterNewFunc(c.name+".ensure", func(ctx context.Context) error { return c.ensure() })
	g.Engine.RegisterNewFunc(c.name+".dispense", func(ctx context.Context) error { return c.dispense() })
	g.Engine.RegisterNewFunc(c.name+".light_on", func(ctx context.Context) error { return c.lightOn() })
	g.Engine.RegisterNewFunc(c.name+".light_off", func(ctx context.Context) error { return c.lightOff() })
	g.Engine.RegisterNewFunc(c.name+".light_on_schedule", func(ctx context.Context) error {
		if !c.lightShouldWork() {
			if c.Light {
				return c.lightOff()
			}
		}
		return c.lightOn()
	})

	err := c.dev.Rst()
	return errors.Annotate(err, c.name+".init")
}

func (c *DeviceCup) lightOn() error {
	c.Light = true
	return c.CommandWaitSuccess(5, 0x02)
}
func (c *DeviceCup) lightOff() error {
	c.dev.Log.Info("light on")
	c.Light = false
	return c.CommandWaitSuccess(5, 0x03)
}
func (c *DeviceCup) dispense() error {
	c.dev.Log.Info("cup dispense")
	return c.CommandWaitSuccess(c.timeout, 0x01)
}
func (c *DeviceCup) ensure() error { return c.CommandWaitSuccess(c.timeout, 0x04) }

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
	wd := `([0-6]|\*)[-]?([0-6])? ([01][0-9]|2[0-3]):([0-5][0-9])-([01][0-9]|2[0-3]):([0-5][0-9])`
	cmd := regexp.MustCompile(wd)

	parts := cmd.FindAllStringSubmatch(sh, 7)
	if (sh != "") && (len(parts) == 0) {
		c.dev.Log.Errorf("shedule string error: %s", sh)
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

// func (s *DeviceCup) textTimeToDudation(week int, v []string) {
// 	h, _ := strconv.Atoi(v[3])
// 	m, _ := strconv.Atoi(v[4])
// 	s.lightShedule.weekDay[week].BeginOfWork = time.Hour*time.Duration(h) + time.Minute*time.Duration(m)
// 	h, _ = strconv.Atoi(v[5])
// 	m, _ = strconv.Atoi(v[6])
// 	s.lightShedule.weekDay[week].EndOfWork = time.Hour*time.Duration(h) + time.Minute*time.Duration(m)
// }

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
