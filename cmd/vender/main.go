package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	cmd_engine "github.com/AlexTransit/vender/cmd/vender/engine"
	"github.com/AlexTransit/vender/cmd/vender/mdb"
	"github.com/AlexTransit/vender/cmd/vender/subcmd"
	cmd_tele "github.com/AlexTransit/vender/cmd/vender/tele"
	"github.com/AlexTransit/vender/cmd/vender/ui"
	"github.com/AlexTransit/vender/cmd/vender/vmc"
	"github.com/AlexTransit/vender/internal/broken"
	"github.com/AlexTransit/vender/internal/state"
	state_new "github.com/AlexTransit/vender/internal/state/new"
	"github.com/AlexTransit/vender/internal/tele"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/log2"
)

var (
	log     = log2.NewStderr(log2.LOG_DEBUG)
	modules = []subcmd.Mod{
		// vmc.BrokenMod,
		cmd_engine.Mod,
		mdb.Mod,
		cmd_tele.Mod,
		ui.Mod,
		vmc.VmcMod,
		vmc.CmdMod,
		{Name: "version", Main: versionMain},
	}
)

var (
	BuildVersion  string = "unknown" // set by ldflags -X
	reFlagVersion        = regexp.MustCompile("-?-?version")
)

func main() {
	// log.SetFlags(0)

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprint(flagset.Output(), "Usage: [option...] command\n\nOptions:\n")
		flagset.PrintDefaults()
		commandNames := make([]string, len(modules))
		for i, m := range modules {
			commandNames[i] = m.Name
		}
		fmt.Fprintf(flagset.Output(), "Commands: %s\n", strings.Join(commandNames, " "))
	}
	configPath := flagset.String("config", "vender.hcl", "")
	onlyVersion := flagset.Bool("version", false, "print build version and exit")
	if err := flagset.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
	if *onlyVersion || reFlagVersion.MatchString(flagset.Arg(0)) {
		versionMain(context.Background(), nil)
		return
	}

	mod, err := subcmd.Parse(flagset.Arg(0), modules)
	if err != nil {
		fmt.Fprintf(flagset.Output(), "command line error: %v\n\n", err)
		flagset.Usage()
		os.Exit(0)
	}
	log.SetFlags(log2.LServiceFlags)
	if !subcmd.SdNotify("start") {
		// under systemd assume systemd journal logging, no timestamp
		log.LogToConsole()
		log.SetFlags(log2.LInteractiveFlags)
	} else {
		log.LogToSyslog(mod.Name)
	}

	config := state.MustReadConfig(log, state.NewOsFullReader(), *configPath)
	ctx, g := state_new.NewContext(log, tele.New())
	g.BuildVersion = BuildVersion
	broken.BrokenInit(g)
	types.Log = log
	log.Debugf("starting command %s", mod.Name)
	if err := mod.Main(ctx, config, flagset.Args()); err != nil {
		g.Log.Errorf("%v", err)
	}
}

func versionMain(ctx context.Context, config *state.Config, _ ...[]string) error {
	fmt.Printf("vender %s\n", BuildVersion)
	return nil
}
