// Main, user facing mode of operation.
package vmc

import (
	"context"
	"fmt"
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
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/coreos/go-systemd/daemon"
	"github.com/juju/errors"
)

var (
	VmcMod = subcmd.Mod{Name: "vmc", Main: VmcMain}
	CmdMod = subcmd.Mod{Name: "cmd", Main: CmdMain}
)

func VmcMain(ctx context.Context, args ...[]string) error {
	g := state.GetGlobal(ctx)
	if watchdog.IsBroken() {
		broken(ctx)
	}
	sound.Init(&g.Config.Sound, g.Log, g.Alive, true)
	err := g.Init(ctx, g.Config)
	if err != nil {
		g.Fatal(err)
	}
	// working term signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT)
	go func() {
		sig := <-sigs
		m := fmt.Sprintf("system signal - %v", sig)
		g.Log.Info(m)
		g.VmcStop(ctx, m)
	}()
	subcmd.SdNotify(daemon.SdNotifyReady)

	display := g.MustTextDisplay()
	display.SetLine(1, "boot "+g.BuildVersion)

	mdbus, err := g.Mdb()
	if mdbus != nil {
		if err != nil {
			return errors.Annotate(err, "mdb init")
		}
		if err = mdbus.ResetDefault(); err != nil {
			return errors.Annotate(err, "mdb bus reset")
		}

		if err = hardware.InitMDBDevices(ctx); err != nil {
			return errors.Annotate(err, "hardware init")
		}
	}

	moneysys := new(money.MoneySystem)
	if err := moneysys.Start(ctx); err != nil {
		return errors.Annotate(err, "money system Start()")
	}

	ui := ui.UI{}
	if err := ui.Init(ctx); err != nil {
		return errors.Annotate(err, "ui Init()")
	}
	g.Log.Debugf("VMC init complete")

	ui.Loop(ctx)
	return nil
}

func CmdMain(ctx context.Context, a ...[]string) error {
	g := state.GetGlobal(ctx)
	if len(a[0]) <= 1 {
		a[0] = append(a[0], "help")
		// g.Log.Infof("command %v - error. few parameters", a)
		// os.Exit(0)
	}
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
	case "help":
		showHelpCMD()
	case "sound":
		sound.Init(&g.Config.Sound, g.Log, g.Alive, false)
		sound.PlayFile(args[1])
		os.Exit(0)
	case "text":
		showText(ctx, a[0][2:])
		os.Exit(0)
	case "broken":
		broken(ctx)
	case "no_init":
		watchdog.SetDeviceInited()
	case "exitcode":
		if len(args) < 3 || args[2] != "success" {
			g.Tele.Init(ctx, g.Log, g.Config.Tele, g.BuildVersion)
			g.Tele.ErrorStr(fmt.Sprintf("exit code %v", args))
			g.RunBashSript(g.Config.ScriptIfBroken)
		}
		if args[1] == "0" {
			g.Log.Info("exit code 0")
			os.Exit(0)
		}
		broken(ctx)
	default:
		return nil
	}

	return nil
}

func showText(ctx context.Context, s []string) {
	var l1, l2 string
	if cap(s) >= 1 {
		l1 = s[0]
	}
	if cap(s) >= 2 {
		l2 = s[1]
	}
	g := state.GetGlobal(ctx)
	display := g.MustTextDisplay()
	display.SetLines(l1, l2)
}

func broken(ctx context.Context) {
	watchdog.SetBroken()
	g := state.GetGlobal(ctx)
	g.Tele.Init(ctx, g.Log, g.Config.Tele, g.BuildVersion)
	sound.Init(&g.Config.Sound, g.Log, g.Alive, false)
	g.SendNotWork(tele_api.State_Broken)
	g.Display()
	display := g.MustTextDisplay()
	// FIXME alexm
	display.SetLine(1, "ABTOMAT")
	display.SetLine(2, "HE ABTOMAT :(")
	g.RunBashSript(g.Config.ScriptIfBroken)
	// FIXME alexm
	sound.PlayFile("broken.mp3")
	for {
		time.Sleep(time.Second)
	}
}

func showHelpCMD() {
	fmt.Println("\n vender cmd sound - play file from /audio directory (mono 24000Hz)")
	fmt.Println("vender cmd text line1_text line2_text (use _ instead space)")
	fmt.Println("vender cmd broken - broken mode")
	fmt.Println("vender cmd no_init - not relecase cup after start")
	fmt.Println("vender cmd exitcode $EXIT_STATUS $SERVICE_RESULT - use systemd service exit code and exit result. if result not `success` the script_if_broken in the config will run")
}
