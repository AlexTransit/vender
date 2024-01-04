// Main, user facing mode of operation.
package vmc

import (
	"context"

	"github.com/AlexTransit/vender/cmd/vender/subcmd"
	"github.com/AlexTransit/vender/hardware"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/ui"
	"github.com/AlexTransit/vender/internal/watchdog"
	"github.com/coreos/go-systemd/daemon"
	"github.com/juju/errors"
)

var (
	VmcMod    = subcmd.Mod{Name: "vmc", Main: VmcMain}
	BrokenMod = subcmd.Mod{Name: "broken", Main: BrokenMain}
)

func VmcMain(ctx context.Context, config *state.Config) error {
	g := state.GetGlobal(ctx)
	g.MustInit(ctx, config)

	display := g.MustTextDisplay()
	display.SetLines("boot "+g.BuildVersion, g.Config.UI.Front.MsgWait)

	mdbus, err := g.Mdb()
	if err != nil {
		return errors.Annotate(err, "mdb init")
	}
	if err = mdbus.ResetDefault(); err != nil {
		return errors.Annotate(err, "mdb bus reset")
	}

	if err = hardware.InitMDBDevices(ctx); err != nil {
		return errors.Annotate(err, "hardware init")
	}

	moneysys := new(money.MoneySystem)
	if err := moneysys.Start(ctx); err != nil {
		return errors.Annotate(err, "money system Start()")
	}

	ui := ui.UI{}
	if err := ui.Init(ctx); err != nil {
		return errors.Annotate(err, "ui Init()")
	}
	watchdog.Init(&config.Watchdog, g.Log)

	subcmd.SdNotify(daemon.SdNotifyReady)
	g.Log.Debugf("VMC init complete")

	ui.Loop(ctx)
	return nil
}

func BrokenMain(ctx context.Context, config *state.Config) error {
	g := state.GetGlobal(ctx)
	g.MustInit(ctx, config)

	display := g.MustTextDisplay()
	display.SetLines("boot "+g.BuildVersion, g.Config.UI.Front.MsgWait)

	subcmd.SdNotify(daemon.SdNotifyReady)

	if mdbus, err := g.Mdb(); err != nil || mdbus == nil {
		if err == nil {
			err = errors.Errorf("hardware problem, see logs")
		}
		err = errors.Annotate(err, "mdb init")
		g.Error(err)
	} else {
		if err = mdbus.ResetDefault(); err != nil {
			err = errors.Annotate(err, "mdb bus reset")
			g.Error(err)
		}
		if err = hardware.InitMDBDevices(ctx); err != nil {
			err = errors.Annotate(err, "hardware enum")
			g.Error(err)
		}
		moneysys := new(money.MoneySystem)
		if err := moneysys.Start(ctx); err != nil {
			err = errors.Annotate(err, "money system Start()")
			g.Error(err)
		} else {
			g.Error(moneysys.ReturnMoney())
		}
	}

	g.Tele.RoboSendBroken()
	display.SetLines(g.Config.UI.Front.MsgBrokenL1, g.Config.UI.Front.MsgBrokenL2)
	g.Error(errors.Errorf("critical daemon broken mode"))
	g.Alive.Wait()
	return nil
}
