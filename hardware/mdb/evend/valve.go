package evend

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/internal/state"
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
	DoSetTempHot    engine.FuncArg
	DoPourCold      engine.Doer
	DoPourHot       engine.Doer
	DoPourEspresso  engine.Doer
	tempHot         int32
	tempHotTarget   uint8
	tempHotReported bool
}

var EValve DeviceValve

func (dv *DeviceValve) init(ctx context.Context) (err error) {
	g := state.GetGlobal(ctx)
	valveConfig := &g.Config.Hardware.Evend.Valve
	dv.pourTimeout = helpers.IntSecondDefault(valveConfig.PourTimeoutSec, 10*time.Minute) // big default timeout is fine, depend on valve hardware
	dv.proto2BusyMask = valvePollBusy
	dv.proto2IgnoreMask = valvePollNotHot
	dv.Generic.Init(ctx, 0xc0, "valve", proto2)
	dv.doGetTempHot = dv.newGetTempHot()
	dv.DoPourCold = dv.newPourCold()
	dv.DoPourHot = dv.newPourHot()
	dv.DoPourEspresso = dv.newPourEspresso()
	var waterStock *inventory.Stock
	waterStock, err = g.Inventory.Get("water")
	if err == nil {
		g.Engine.Register("add.water_hot(?)", waterStock.Wrap(dv.DoPourHot))
		g.Engine.Register("add.water_cold(?)", waterStock.Wrap(dv.DoPourCold))
		g.Engine.Register("add.water_espresso(?)", waterStock.Wrap(dv.DoPourEspresso))
		dv.cautionPartUnit = uint8(waterStock.TranslateHw(engine.Arg(valveConfig.CautionPartMl)))
	} else {
		dv.dev.Log.Errorf("invalid config, stock water not found err=%v", err)
	}
	g.Engine.RegisterNewFunc("evend.valve.get_temp_hot", func(ctx context.Context) error { return dv.readTemp() })
	g.Engine.RegisterNewFunc("evend.valve.reset", func(ctx context.Context) error { return dv.dev.Rst() })
	g.Engine.Register("evend.valve.set_temp_hot(?)",
		engine.FuncArg{Name: "evend.valve.set_temp_hot", F: func(ctx context.Context, arg engine.Arg) error {
			dv.SetTemp(uint8(arg))
			return nil
		}})
	g.Engine.RegisterNewFunc("evend.valve.set_temp_hot_config", func(ctx context.Context) error { return dv.SetTemp(uint8(valveConfig.TemperatureHot)) })
	g.Engine.RegisterNewFunc("evend.valve.status", func(ctx context.Context) error {
		ct, err := dv.GetTemperature()
		g.Log.Infof("current temp=%v target temp=%v", ct, EValve.tempHotTarget)
		return err
	})
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
	err = dv.dev.Rst()
	dv.poll()
	return err
}

func (dv *DeviceValve) GetTemperature() (int32, error) {
	if e := dv.readTemp(); e != nil {
		e = errors.Join(e, dv.dev.Rst())
		return -1, e
	}
	return dv.tempHot, nil
}

func (dv *DeviceValve) readTemp() (err error) {
	tag := dv.name + ".get_temp_hot"
	bs := []byte{dv.dev.Address + 4, 0x11}
	request := mdb.MustPacketFromBytes(bs, true)
	response := mdb.Packet{}
	err = dv.Generic.dev.Tx(request, &response)
	if err != nil {
		return fmt.Errorf("%s %v", tag, err)
	}
	bs = response.Bytes()
	dv.dev.Log.Debugf("%s request=%s response=(%d)%s", tag, request.Format(), response.Len(), response.Format())
	if len(bs) != 1 {
		return fmt.Errorf("%s response=%x", tag, bs)
	}
	temp := int32(bs[0])
	// AlexM FIXME  (temp != 255) - затычка
	if temp >= 100 && temp != 255 {
		dv.dev.Rst()
		return fmt.Errorf("overhead temp=%v", temp)
	}
	defer func() {
		dv.tempHot = temp
	}()
	if temp == 0 {
		if err = dv.getError(); err != nil {
			temp = -1
			return err
		}
	}
	return nil
}

func (dv *DeviceValve) getError() error {
	bs := []byte{dv.dev.Address + 4, 0x02}
	request := mdb.MustPacketFromBytes(bs, true)
	response := mdb.Packet{}
	err := dv.Generic.dev.Tx(request, &response)
	if err != nil {
		return fmt.Errorf("error read error :) err=%v", err)
	}
	if response.Bytes()[0] == 0 {
		return nil
	}
	return fmt.Errorf("valve error=%v", response.Bytes()[0])
}

func (dv *DeviceValve) SetTemp(temp uint8) (err error) {
	if e := dv.setT(temp); e != nil {
		defer dv.dev.Log.Err(err)
		err = fmt.Errorf("set tempeture:%v error(%v)", temp, e)
		time.Sleep(3 * time.Second)
		if e := dv.setT(temp); e != nil {
			em := fmt.Errorf("two time set tempeture:%v error(%v)", temp, e)
			err = errors.Join(err, em)
			return err
		}
	}
	return nil
}

func (dv *DeviceValve) poll() {
	bs := []byte{dv.dev.Address + 3}
	request := mdb.MustPacketFromBytes(bs, true)
	response := mdb.Packet{}
	if err := dv.dev.Tx(request, &response); err != nil {
		dv.dev.Log.WarningF("valve pool error(%v)", err)
	}
	if len(response.Bytes()) != 0 {
		dv.dev.Log.Errorf("init valve pool response not nil (%v)", response.Bytes())
	}
}

func (dv *DeviceValve) setT(temp uint8) (err error) {
	bs := []byte{dv.dev.Address + 5, 0x10, temp}
	request := mdb.MustPacketFromBytes(bs, true)
	response := mdb.Packet{}
	err = dv.dev.Tx(request, &response)
	dv.tempHotTarget = temp
	return err
}

func (dv *DeviceValve) newGetTempHot() engine.Func {
	tag := dv.name + ".get_temp_hot"
	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		bs := []byte{dv.dev.Address + 4, 0x11}
		request := mdb.MustPacketFromBytes(bs, true)
		response := mdb.Packet{}
		err := dv.Generic.dev.TxKnown(request, &response)
		if err != nil {
			return fmt.Errorf("%s %v", tag, err)
		}
		bs = response.Bytes()
		dv.dev.Log.Debugf("%s request=%s response=(%d)%s", tag, request.Format(), response.Len(), response.Format())
		if len(bs) != 1 {
			return fmt.Errorf("%s response=%x", tag, bs)
		}
		temp := int32(bs[0])
		// if temp == 0 || temp >= 100 {
		if temp >= 100 {
			if doSetZero, _, _ := engine.ArgApply(dv.DoSetTempHot, 0); doSetZero != nil {
				_ = engine.GetGlobal(ctx).Exec(ctx, doSetZero)
			}
			sensorErr := fmt.Errorf("%s current temp=%v", tag, temp)
			return sensorErr
		}
		dv.tempHot = temp
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
				return fmt.Errorf("arg=%d overflows hardware units", arg)
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
		},
	}

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
		temp := dv.tempHot
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
				g.Error(fmt.Errorf(msg))
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
