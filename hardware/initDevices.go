package hardware

import (
	"context"
	"sync"

	"github.com/AlexTransit/vender/hardware/mdb/bill"
	"github.com/AlexTransit/vender/hardware/mdb/coin"
	"github.com/AlexTransit/vender/hardware/mdb/evend"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
)

func InitMDBDevices(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	errch := make(chan error, 16)
	wg := sync.WaitGroup{}

	for _, rd := range g.Config.Hardware.EvendDevices {
		if rd.Disabled {
			continue
		}
		wg.Add(1)
		switch rd.Name {
		case "bill":
			go helpers.WrapErrChan(&wg, errch, func() error { return bill.Enum(ctx) })
		case "coin":
			go coin.InitDevice(ctx)
			wg.Done()
		case "evend.conveyor":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumConveyor(ctx) })
		case "evend.cup":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumCup(ctx) })
		case "evend.elevator":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumElevator(ctx) })
		case "evend.espresso":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumEspresso(ctx) })
		case "evend.mixer":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumMixer(ctx) })
		case "evend.valve":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumValve(ctx) })
		case "evend.multihopper":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumMultiHopper(ctx) })
		case "evend.hopper1":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 1) })
		case "evend.hopper2":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 2) })
		case "evend.hopper3":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 3) })
		case "evend.hopper4":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 4) })
		case "evend.hopper5":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 5) })
		case "evend.hopper6":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 6) })
		case "evend.hopper7":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 7) })
		case "evend.hopper8":
			go helpers.WrapErrChan(&wg, errch, func() error { return evend.EnumHopper(ctx, 8) })
		default:
			wg.Done()
		}
	}
	wg.Wait()
	errch <- g.CheckDevices()
	close(errch)
	return helpers.FoldErrChan(errch)
}
