// Main, user facing mode of operation.
package vmc

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/AlexTransit/vender/cmd/vender/subcmd"
	"github.com/AlexTransit/vender/hardware"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/ui"
	"github.com/AlexTransit/vender/internal/watchdog"
	"github.com/coreos/go-systemd/daemon"
	"github.com/juju/errors"
)

var (
	VmcMod = subcmd.Mod{Name: "vmc", Main: VmcMain}
	// BrokenMod = subcmd.Mod{Name: "broken", Main: BrokenMain}
	CmdMod = subcmd.Mod{Name: "cmd", Main: CmdMain}
)

func VmcMain(ctx context.Context, config *state.Config, args ...[]string) error {
	g := state.GetGlobal(ctx)
	subcmd.SdNotify(daemon.SdNotifyReady)
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

	// subcmd.SdNotify(daemon.SdNotifyReady)
	g.Log.Debugf("VMC init complete")

	ui.Loop(ctx)
	return nil
}

// func BrokenMain(ctx context.Context, config *state.Config, args []string) error {
// 	g := state.GetGlobal(ctx)
// 	g.MustInit(ctx, config)

// 	display := g.MustTextDisplay()
// 	display.SetLines("boot "+g.BuildVersion, g.Config.UI.Front.MsgWait)

// 	subcmd.SdNotify(daemon.SdNotifyReady)

// 	if mdbus, err := g.Mdb(); err != nil || mdbus == nil {
// 		if err == nil {
// 			err = errors.Errorf("hardware problem, see logs")
// 		}
// 		err = errors.Annotate(err, "mdb init")
// 		g.Error(err)
// 	} else {
// 		if err = mdbus.ResetDefault(); err != nil {
// 			err = errors.Annotate(err, "mdb bus reset")
// 			g.Error(err)
// 		}
// 		if err = hardware.InitMDBDevices(ctx); err != nil {
// 			err = errors.Annotate(err, "hardware enum")
// 			g.Error(err)
// 		}
// 		moneysys := new(money.MoneySystem)
// 		if err := moneysys.Start(ctx); err != nil {
// 			err = errors.Annotate(err, "money system Start()")
// 			g.Error(err)
// 		} else {
// 			g.Error(moneysys.ReturnMoney())
// 		}
// 	}

// 	g.Tele.RoboSendBroken()
// 	display.SetLines(g.Config.UI.Front.MsgBrokenL1, g.Config.UI.Front.MsgBrokenL2)
// 	g.Error(errors.Errorf("critical daemon broken mode"))
// 	g.Alive.Wait()
// 	return nil
// }

func CmdMain(ctx context.Context, config *state.Config, a ...[]string) error {
	g := state.GetGlobal(ctx)
	subcmd.SdNotify(daemon.SdNotifyReady)

	args := a[0][1:]
	switch strings.ToLower(args[0]) {
	case "exitcode":
		if args[1] == "0" {
			g.Log.Info("exit code 0")
			os.Exit(0)
		}
		broken(ctx, config)
	default:
		return nil
	}

	return nil
}

func broken(ctx context.Context, config *state.Config) {
	g := state.GetGlobal(ctx)
	sound.Init(&config.Sound, g.Log, false)
	sound.Broken()
	time.Sleep(2 * time.Second)
	sound.Stop()
	for {
		time.Sleep(time.Second)
	}
}
