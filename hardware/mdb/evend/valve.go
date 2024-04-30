package evend

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

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

	tempHot       int32
	tempHotTarget uint8
	waterStock    *inventory.Stock
}

var EValve DeviceValve

func (dv *DeviceValve) init(ctx context.Context) (err error) {
	g := state.GetGlobal(ctx)
	valveConfig := &g.Config.Hardware.Evend.Valve
	dv.proto2BusyMask = valvePollBusy
	dv.proto2IgnoreMask = valvePollNotHot
	dv.Generic.Init(ctx, 0xc0, "valve", proto2)
	dv.waterStock, err = g.Inventory.Get("water")
	if err != nil {
		dv.waterStock = &inventory.Stock{}
	}
	g.Engine.RegisterNewFuncAgr("add.water_hot(?)", func(ctx context.Context, arg engine.Arg) error { return dv.waterRun(waterHot, uint8(arg)) })
	g.Engine.RegisterNewFuncAgr("add.water_cold(?)", func(ctx context.Context, arg engine.Arg) error { return dv.waterRun(waterCold, uint8(arg)) })
	g.Engine.RegisterNewFuncAgr("add.water_espresso(?)", func(ctx context.Context, arg engine.Arg) error { return dv.waterRun(waterEspresso, uint8(arg)) })
	g.Engine.RegisterNewFunc("evend.valve.get_temp_hot", func(ctx context.Context) error { return dv.readTemp() })
	g.Engine.RegisterNewFunc("evend.valve.reset", func(ctx context.Context) error { return dv.Reset() })

	g.Engine.RegisterNewFuncAgr("evend.valve.set_temp_hot(?)", func(ctx context.Context, arg engine.Arg) error { return dv.SetTemp(uint8(arg)) })

	g.Engine.RegisterNewFunc("evend.valve.set_temp_hot_config", func(ctx context.Context) error { return dv.SetTemp(uint8(valveConfig.TemperatureHot)) })
	g.Engine.RegisterNewFunc("evend.valve.status", func(ctx context.Context) error {
		ct, err := dv.GetTemperature()
		g.Log.Infof("current temp=%v target temp=%v", ct, EValve.tempHotTarget)
		return err
	})
	g.Engine.RegisterNewFunc("evend.valve.cold_open", func(ctx context.Context) error { return dv.Command(0x10, 0x01) })
	g.Engine.RegisterNewFunc("evend.valve.cold_close", func(ctx context.Context) error { return dv.Command(0x10, 0x00) })
	g.Engine.RegisterNewFunc("evend.valve.hot_open", func(ctx context.Context) error { return dv.Command(0x11, 0x01) })
	g.Engine.RegisterNewFunc("evend.valve.hot_close", func(ctx context.Context) error { return dv.Command(0x11, 0x00) })
	g.Engine.RegisterNewFunc("evend.valve.boiler_start", func(ctx context.Context) error { return dv.Command(0x12, 0x01) })
	g.Engine.RegisterNewFunc("evend.valve.boiler_stop", func(ctx context.Context) error { return dv.Command(0x12, 0x00) })
	g.Engine.RegisterNewFunc("evend.valve.pump_espresso_start", func(ctx context.Context) error { return dv.Command(0x13, 0x01) })
	g.Engine.RegisterNewFunc("evend.valve.pump_espresso_stop", func(ctx context.Context) error { return dv.Command(0x13, 0x00) })
	g.Engine.RegisterNewFunc("evend.valve.reserved_on", func(ctx context.Context) error { return dv.Command(0x14, 0x01) })
	g.Engine.RegisterNewFunc("evend.valve.reserved_off", func(ctx context.Context) error { return dv.Command(0x14, 0x00) })

	dv.Reset()
	return err
}

const (
	waterHot      = byte(0x01)
	waterCold     = byte(0x02)
	waterEspresso = byte(0x03)
)

func (dv *DeviceValve) Reset() (err error) {
	err = dv.dev.Rst()
	if err != nil {
		return
	}
	time.Sleep(3 * time.Second)
	dv.WaitSuccess(1, false)
	dv.SetTemp(0)
	dv.readTemp()
	return dv.WaitSuccess(1, false)
}

func (dv *DeviceValve) waterRun(waterType byte, steps uint8) (err error) {
	dv.log.Infof("start water(%d) (%d)ml.", waterType, steps)
	milliliterToHW := byte(math.Round(float64(dv.waterStock.GetSpendRate() * float32(steps))))
	err = dv.CommandWaitSuccess(uint16(steps*10), waterType, milliliterToHW)
	// err = dv.CommandNoWait(waterType, milliliterToHW)
	dv.log.Infof("stop water(%d) (%d)ml.", waterType, steps)
	dv.waterStock.SpendValue(milliliterToHW)
	return err
}

func (dv *DeviceValve) GetTemperature() (int32, error) {
	if e := dv.readTemp(); e != nil {
		e = errors.Join(e, dv.dev.Rst())
		return -1, e
	}
	return dv.tempHot, nil
}

func (dv *DeviceValve) readTemp() error {
	r, err := dv.ReadData(0x11)
	if err != nil {
		dv.tempHot = -1
		return err
	}
	dv.tempHot = int32(r[0])
	if dv.tempHot == 0 || dv.tempHot > 100 {
		return errors.Join(fmt.Errorf("invalid temp=%v", dv.tempHot), dv.ReadError())
	}
	return nil
}

func (dv *DeviceValve) SetTemp(temp uint8) (err error) {
	err = dv.SetConfig(0x10, temp)
	if err == nil {
		dv.tempHotTarget = temp
		return
	}
	dv.tempHotTarget = 0
	return
}
