package evend

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	// "github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/juju/errors"
)

const DefaultCupAssertBusyDelay = 30 * time.Millisecond
const DefaultCupDispenseTimeout = 30 * time.Second
const DefaultCupEnsureTimeout = 70 * time.Second

type DeviceCup struct {
	Generic
	d                   *engine.Engine
	dispenseTimeout     time.Duration
	assertBusyDelayMils time.Duration
	Light               bool
	lightShedule        struct {
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

func (devCup *DeviceCup) init(ctx context.Context) error {
	devCup.Generic.Init(ctx, 0xe0, "cup", proto2)

	g := state.GetGlobal(ctx)
	devCup.initLightSheduler(g.Config.UI.Front.LightShedule)
	devCup.dispenseTimeout = helpers.IntSecondDefault(g.Config.Hardware.Evend.Cup.DispenseTimeoutSec, DefaultCupDispenseTimeout)
	devCup.assertBusyDelayMils = helpers.IntMillisecondDefault(g.Config.Hardware.Evend.Cup.AssertBusyDelayMs, DefaultCupAssertBusyDelay)
	devCup.d = g.Engine
	// doDispense := devCup.Generic.WithRestart(devCup.NewDispenseProper())
	g.Engine.Register(devCup.name+".dispense", devCup.WithRestart(devCup.NewDispenseProper()))
	g.Engine.Register(devCup.name+".light_on", devCup.DevLight(ctx, true))
	g.Engine.Register(devCup.name+".light_off", devCup.DevLight(ctx, false))
	g.Engine.Register(devCup.name+".light_on_schedule",
		engine.Func0{F: func() error {
			if !devCup.lightShouldWork() {
				if devCup.Light {
					err := devCup.d.Exec(ctx, devCup.DevLight(ctx, false))
					return err
				}
				return nil
			}
			err := devCup.d.Exec(ctx, devCup.DevLight(ctx, true))
			return err
		}})

	g.Engine.Register(devCup.name+".ensure", devCup.NewEnsure())

	err := devCup.dev.Rst()
	return errors.Annotate(err, devCup.name+".init")
}

func (devCup *DeviceCup) NewDispenseProper() engine.Doer {
	return engine.NewSeq(devCup.name + ".dispense_proper").
		// Append(devCup.NewEnsure()).
		Append(devCup.NewDispense())
}

func (devCup *DeviceCup) NewDispense() engine.Doer {
	tag := devCup.name + ".dispense"
	return engine.NewSeq(tag).
		Append(engine.Func0{F: func() error { devCup.dev.Log.Info("cup dispence"); return nil }}).
		Append(devCup.NewWaitReady(tag)).
		Append(devCup.NewAction(tag, 0x01))
	// Append(devCup.NewWaitDone(tag, devCup.dispenseTimeout))
}

func (devCup *DeviceCup) DevLight(ctx context.Context, v bool) engine.Doer {
	return engine.Func0{F: func() error {
		if devCup.Light == v {
			return nil
		}
		devCup.Light = v
		tag := fmt.Sprintf("%s.light:%t", devCup.name, v)
		devCup.dev.Log.Infof(tag)
		arg := byte(0x02)
		if !v {
			arg = 0x03
		}
		err := devCup.d.Exec(ctx, devCup.NewAction(tag, arg))
		return err
	}}
}

func (devCup *DeviceCup) NewEnsure() engine.Doer {
	tag := devCup.name + ".ensure"
	return engine.NewSeq(tag).
		Append(devCup.NewWaitReady(tag)).
		Append(devCup.NewAction(tag, 0x04)).
		Append(devCup.NewWaitDone(tag, devCup.dispenseTimeout))
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
func (devCup *DeviceCup) initLightSheduler(sh string) {
	wd := `([0-6]|\*)[-]?([0-6])? ([01][0-9]|2[0-3]):([0-5][0-9])-([01][0-9]|2[0-3]):([0-5][0-9])`
	cmd := regexp.MustCompile(wd)

	parts := cmd.FindAllStringSubmatch(sh, 7)
	if (sh != "") && (len(parts) == 0) {
		devCup.dev.Log.Errorf("shedule string error: %s", sh)
		return
	}
	for _, v := range parts {
		devCup.dev.Log.Infof("add light shedule %v", v[0])
		switch v[1] {
		case "*":
			for i := 0; i < 7; i++ {
				devCup.writeShedule(i, v)
			}
		default:
			w, _ := strconv.Atoi(v[1])
			if v[2] == "" {
				devCup.writeShedule(w, v)
			} else {
				e, _ := strconv.Atoi(v[2])
				for i := w; i <= e; i++ {
					devCup.writeShedule(i, v)
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
