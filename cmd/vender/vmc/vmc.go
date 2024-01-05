// Main, user facing mode of operation.
package vmc

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	g.MustInit(ctx, config)

	// working term signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		g.Log.Infof("system signal - %v", sig)
		g.VmcStop(ctx)
	}()
	subcmd.SdNotify(daemon.SdNotifyReady)

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

func CmdMain(ctx context.Context, config *state.Config, a ...[]string) error {
	g := state.GetGlobal(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		g.Log.Infof("system signal - %v", sig)
		os.Exit(0)
	}()
	subcmd.SdNotify(daemon.SdNotifyReady)

	args := a[0][1:]
	switch strings.ToLower(args[0]) {
	case "aa":
		sound.Init(&config.Sound, g.Log, false)
		sound.PlayFile("moneyIn.mp3")
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "broken":
		broken(ctx, config)
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
