package evend

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/helpers/cacheval"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/juju/errors"
)

const (
	valvePollBusy   = 0x10
	valvePollNotHot = 0x40
)

type ErrWaterTemperature struct {
	Source  string
	Current int16
	Target  int16
}

func (e *ErrWaterTemperature) Error() string {
	diff := e.Current - e.Target
	return fmt.Sprintf("source=%s current=%d target=%d diff=%d", e.Source, e.Current, e.Target, diff)
}

type DeviceValve struct { //nolint:maligned
	Generic

	cautionPartUnit uint8
	pourTimeout     time.Duration

	doGetTempHot    engine.Doer
	doCheckTempHot  engine.Doer
	DoSetTempHot    engine.FuncArg
	DoPourCold      engine.Doer
	DoPourHot       engine.Doer
	DoPourEspresso  engine.Doer
	tempHot         cacheval.Int32
	tempHotTarget   uint8
	tempHotReported bool
}

func (dv *DeviceValve) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	valveConfig := &g.Config.Hardware.Evend.Valve
	dv.pourTimeout = helpers.IntSecondDefault(valveConfig.PourTimeoutSec, 10*time.Minute) // big default timeout is fine, depend on valve hardware
	tempValid := helpers.IntMillisecondDefault(valveConfig.TemperatureValidMs, 30*time.Second)
	dv.tempHot.Init(tempValid)
	dv.proto2BusyMask = valvePollBusy
	dv.proto2IgnoreMask = valvePollNotHot
	dv.Generic.Init(ctx, 0xc0, "valve", proto2)

	dv.doGetTempHot = dv.newGetTempHot()
	dv.doCheckTempHot = engine.Func0{F: func() error { return nil }, V: dv.newCheckTempHotValidate(ctx)}
	dv.DoSetTempHot = dv.newSetTempHot()
	dv.DoPourCold = dv.newPourCold()
	dv.DoPourHot = dv.newPourHot()
	dv.DoPourEspresso = dv.newPourEspresso()

	waterStock, err := g.Inventory.Get("water")
	if err == nil {
		g.Engine.Register("add.water_hot(?)", waterStock.Wrap(dv.DoPourHot))
		g.Engine.Register("add.water_cold(?)", waterStock.Wrap(dv.DoPourCold))
		g.Engine.Register("add.water_espresso(?)", waterStock.Wrap(dv.DoPourEspresso))
		dv.cautionPartUnit = uint8(waterStock.TranslateHw(engine.Arg(valveConfig.CautionPartMl)))
	} else {
		dv.dev.Log.Errorf("invalid config, stock water not found err=%v", err)
	}

	g.Engine.Register("evend.valve.check_temp_hot", dv.doCheckTempHot)
	g.Engine.Register("evend.valve.get_temp_hot", dv.doGetTempHot)
	g.Engine.Register("evend.valve.set_temp_hot(?)", dv.DoSetTempHot)
	g.Engine.Register("evend.valve.set_temp_hot_config", engine.Func{F: func(ctx context.Context) error {
		d, _, err := dv.DoSetTempHot.Apply(engine.Arg(valveConfig.TemperatureHot))
		if err != nil {
			return err
		}
		return g.Engine.Exec(ctx, d)
	}})
	g.Engine.Register("evend.valve.pour_espresso(?)", dv.DoPourEspresso.(engine.Doer))
	g.Engine.Register("evend.valve.pour_cold(?)", dv.DoPourCold.(engine.Doer))
	g.Engine.Register("evend.valve.pour_hot(?)", dv.DoPourHot.(engine.Doer))
	g.Engine.Register("evend.valve.cold_open", dv.NewValveCold(true))
	g.Engine.Register("evend.valve.cold_close", dv.NewValveCold(false))
	g.Engine.Register("evend.valve.hot_open", dv.NewValveHot(true))
	g.Engine.Register("evend.valve.hot_close", dv.NewValveHot(false))
	g.Engine.Register("evend.valve.boiler_open", dv.NewValveBoiler(true))
	g.Engine.Register("evend.valve.boiler_close", dv.NewValveBoiler(false))
	g.Engine.Register("evend.valve.pump_espresso_start", dv.NewPumpEspresso(true))
	g.Engine.Register("evend.valve.pump_espresso_stop", dv.NewPumpEspresso(false))
	g.Engine.Register("evend.valve.pump_start", dv.NewPump(true))
	g.Engine.Register("evend.valve.pump_stop", dv.NewPump(false))

	err = dv.Generic.FIXME_initIO(ctx)
	return errors.Annotate(err, dv.name+".init")
}

// func (dv *DeviceValve) UnitToTimeout(unit uint8) time.Duration {
// 	const min = 500 * time.Millisecond
// 	const perUnit = 50 * time.Millisecond // FIXME
// 	return min + time.Duration(unit)*perUnit
// }

func (dv *DeviceValve) newGetTempHot() engine.Func {
	tag := dv.name + ".get_temp_hot"

	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		bs := []byte{dv.dev.Address + 4, 0x11}
		request := mdb.MustPacketFromBytes(bs, true)
		response := mdb.Packet{}
		err := dv.Generic.dev.TxKnown(request, &response)
		if err != nil {
			return errors.Annotate(err, tag)
		}
		bs = response.Bytes()
		dv.dev.Log.Debugf("%s request=%s response=(%d)%s", tag, request.Format(), response.Len(), response.Format())
		if len(bs) != 1 {
			return errors.NotValidf("%s response=%x", tag, bs)
		}

		temp := int32(bs[0])
		if temp == 0 {
			// dv.dev.SetErrorCode(1)
			if doSetZero, _, _ := engine.ArgApply(dv.DoSetTempHot, 0); doSetZero != nil {
				_ = engine.GetGlobal(ctx).Exec(ctx, doSetZero)
			}
			sensorErr := errors.Errorf("%s current=0 sensor problem", tag)
			if !dv.tempHotReported {
				g := state.GetGlobal(ctx)
				g.Error(sensorErr)
				dv.tempHotReported = true
			}
			return sensorErr
		}

		dv.tempHot.Set(temp)
		return nil
	}}
}

func (dv *DeviceValve) newSetTempHot() engine.FuncArg {
	tag := dv.name + ".set_temp_hot"

	return engine.FuncArg{Name: tag, F: func(ctx context.Context, arg engine.Arg) error {
		temp := uint8(arg)
		bs := []byte{dv.dev.Address + 5, 0x10, temp}
		request := mdb.MustPacketFromBytes(bs, true)
		response := mdb.Packet{}
		err := dv.dev.TxCustom(request, &response, mdb.TxOpt{})
		if err != nil {
			return errors.Annotatef(err, "%s target=%d request=%x", tag, temp, request.Bytes())
		}
		dv.tempHotTarget = temp
		dv.dev.Log.Debugf("%s target=%d request=%x response=%x", tag, temp, request.Bytes(), response.Bytes())
		return nil
	}}
}

func (dv *DeviceValve) newPourCareful(name string, arg1 byte, abort engine.Doer) engine.Doer {
	tagPour := "pour_" + name
	tag := fmt.Sprintf("%s.%s", dv.name, tagPour)

	doPour := engine.FuncArg{
		Name: tag + "/careful",
		F: func(ctx context.Context, arg engine.Arg) error {
			if arg >= 256 {
				return errors.Errorf("arg=%d overflows hardware units", arg)
			}
			e := engine.GetGlobal(ctx)
			units := uint8(arg)
			if units > dv.cautionPartUnit {
				d := dv.newCommand(tagPour, strconv.Itoa(int(dv.cautionPartUnit)), arg1, dv.cautionPartUnit)
				if err := e.Exec(ctx, d); err != nil {
					return err
				}
				d = dv.Generic.NewWaitDone(tag, dv.pourTimeout)
				if err := e.Exec(ctx, d); err != nil {
					_ = e.Exec(ctx, abort) // TODO likely redundant
					return err
				}
				units -= dv.cautionPartUnit
			}
			d := dv.newCommand(tagPour, strconv.Itoa(int(units)), arg1, units)
			if err := e.Exec(ctx, d); err != nil {
				return err
			}
			err := e.Exec(ctx, dv.Generic.NewWaitDone(tag, dv.pourTimeout))
			return err
		}}

	return engine.NewSeq(tag).
		Append(dv.Generic.NewWaitReady(tag)).
		Append(doPour)
}

func (dv *DeviceValve) newPourEspresso() engine.Doer {
	return dv.newPourCareful("espresso", 0x03, dv.NewPumpEspresso(false))
}

func (dv *DeviceValve) newPourCold() engine.Doer {
	tag := dv.name + ".pour_cold"
	return engine.NewSeq(tag).
		Append(dv.Generic.NewWaitReady(tag)).
		Append(dv.newPour(tag, 0x02)).
		Append(dv.Generic.NewWaitDone(tag, dv.pourTimeout))
}

func (dv *DeviceValve) newPourHot() engine.Doer {
	tag := dv.name + ".pour_hot"
	return engine.NewSeq(tag).
		Append(dv.Generic.NewWaitReady(tag)).
		Append(dv.newPour(tag, 0x01)).
		Append(dv.Generic.NewWaitDone(tag, dv.pourTimeout))
}

func (dv *DeviceValve) NewValveCold(open bool) engine.Doer {
	if open {
		return dv.newCommand("valve_cold", "open", 0x10, 0x01)
	} else {
		return dv.newCommand("valve_cold", "close", 0x10, 0x00)
	}
}
func (dv *DeviceValve) NewValveHot(open bool) engine.Doer {
	if open {
		return dv.newCommand("valve_hot", "open", 0x11, 0x01)
	} else {
		return dv.newCommand("valve_hot", "close", 0x11, 0x00)
	}
}
func (dv *DeviceValve) NewValveBoiler(open bool) engine.Doer {
	if open {
		return dv.newCommand("valve_boiler", "open", 0x12, 0x01)
	} else {
		return dv.newCommand("valve_boiler", "close", 0x12, 0x00)
	}
}
func (dv *DeviceValve) NewPumpEspresso(start bool) engine.Doer {
	if start {
		return dv.newCommand("pump_espresso", "start", 0x13, 0x01)
	} else {
		return dv.newCommand("pump_espresso", "stop", 0x13, 0x00)
	}
}
func (dv *DeviceValve) NewPump(start bool) engine.Doer {
	if start {
		return dv.newCommand("pump", "start", 0x14, 0x01)
	} else {
		return dv.newCommand("pump", "stop", 0x14, 0x00)
	}
}

func (dv *DeviceValve) newPour(tag string, b1 byte) engine.Doer {
	return engine.FuncArg{
		Name: tag,
		F: func(ctx context.Context, arg engine.Arg) error {
			dv.dev.Log.Debugf("%s arg=%d", tag, arg)
			bs := []byte{b1, uint8(arg)}
			return dv.txAction(bs)
		},
	}
}

func (dv *DeviceValve) newCommand(cmdName, argName string, arg1, arg2 byte) engine.Doer {
	tag := fmt.Sprintf("%s.%s:%s", dv.name, cmdName, argName)
	return dv.Generic.NewAction(tag, arg1, arg2)
}

func (dv *DeviceValve) newCheckTempHotValidate(ctx context.Context) func() error {
	g := state.GetGlobal(ctx)
	return func() error {
		tag := dv.name + ".check_temp_hot"
		var getErr error
		temp := dv.tempHot.GetOrUpdate(func() {
			// Alexm - если отключить давчик температуры, после инита, то ошибок не будет и температура не меняется.
			if getErr = g.Engine.Exec(ctx, dv.doGetTempHot); getErr != nil {
				getErr = errors.Annotate(getErr, tag)
				dv.dev.Log.Error(getErr)
			}
		})
		types.VMC.HW.Temperature = int(temp)
		if getErr != nil {
			if doSetZero, _, _ := engine.ArgApply(dv.DoSetTempHot, 0); doSetZero != nil {
				_ = g.Engine.Exec(ctx, doSetZero)
			}
			return getErr
		}

		diff := absDiffU16(uint16(temp), uint16(dv.tempHotTarget))
		const tempHotMargin = 10 // FIXME margin from config
		msg := fmt.Sprintf("%s current=%d config=%d diff=%d", tag, temp, dv.tempHotTarget, diff)
		dv.dev.Log.Debugf(msg)
		if diff > tempHotMargin {
			if !dv.tempHotReported {
				g.Error(errors.New(msg))
				dv.tempHotReported = true
			}
			return &ErrWaterTemperature{
				Source:  "hot",
				Current: int16(temp),
				Target:  int16(dv.tempHotTarget),
			}
		} else if dv.tempHotReported {
			// TODO report OK
			dv.tempHotReported = false
		}
		return nil
	}
}

func absDiffU16(a, b uint16) uint16 {
	if a >= b {
		return a - b
	}
	return b - a
}
