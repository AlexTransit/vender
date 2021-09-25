package mdb

import (
	"context"
	"strconv"
	"strings"
	"time"

	prompt "github.com/c-bata/go-prompt"
	"github.com/juju/errors"
	"github.com/temoto/vender/cmd/vender/subcmd"
	"github.com/temoto/vender/hardware"
	"github.com/temoto/vender/hardware/mdb"
	"github.com/temoto/vender/helpers/cli"
	"github.com/temoto/vender/internal/engine"
	"github.com/temoto/vender/internal/state"
)

const usage = `syntax: commands separated by whitespace
(main)
- reset    MDB bus reset (TX high for 200ms, wait for 500ms)
- sN       pause N milliseconds
- @XX...   transmit MDB block from hex XX..., show response
- loop=N   repeat N times all commands on this line
`

const modName = "mdb-cli"

var Mod = subcmd.Mod{
	Name: modName,
	Main: Main,
}

// cmdline := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
// devicePath := cmdline.String("device", "/dev/ttyAMA0", "")
// iodinPath := cmdline.String("iodin", "./iodin", "Path to iodin executable")
// megaSpi := cmdline.String("mega-spi", "", "mega SPI port")
// megaPin := cmdline.String("mega-pin", "25", "mega notify pin")
// uarterName := cmdline.String("io", "file", "file|iodin|mega")

func Main(ctx context.Context, config *state.Config) error {
	g := state.GetGlobal(ctx)
	g.MustInit(ctx, config)

	synthConfig := &state.Config{}
	synthConfig.Hardware.XXX_Devices = nil
	synthConfig.Hardware.IodinPath = config.Hardware.IodinPath // *iodinPath
	synthConfig.Hardware.Mdb = config.Hardware.Mdb             // *uarterName *devicePath
	synthConfig.Hardware.Mega = config.Hardware.Mega           // *megaSpi *megaPin
	synthConfig.Tele.Enabled = false
	g.MustInit(ctx, synthConfig)

	if _, err := g.Mdb(); err != nil {
		g.Log.Fatal(err)
	}
	defer g.Hardware.Mdb.Uarter.Close()

	if err := g.Engine.ValidateExec(ctx, doBusReset); err != nil {
		g.Log.Fatal(err)
	}

	if err := hardware.Enum(ctx); err != nil {
		err = errors.Annotate(err, "hardware enum")
		return err
	}

	cli.MainLoop(modName, newExecutor(ctx), newCompleter(ctx))
	return nil
}

var doBusReset = engine.Func{Name: "reset", F: func(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	m, err := g.Mdb()
	if err != nil {
		return err
	}
	return m.ResetDefault()
}}

func newCompleter(ctx context.Context) func(d prompt.Document) []prompt.Suggest {
	suggests := []prompt.Suggest{
		prompt.Suggest{Text: "reset", Description: "MDB bus reset"},
		prompt.Suggest{Text: "sN", Description: "pause for N ms"},
		prompt.Suggest{Text: "loop=N", Description: "repeat line N times"},
		prompt.Suggest{Text: "@XX", Description: "transmit MDB block, show response"},
	}

	return func(d prompt.Document) []prompt.Suggest {
		return prompt.FilterFuzzy(suggests, d.GetWordBeforeCursor(), true)
	}
}

func newExecutor(ctx context.Context) func(string) {
	g := state.GetGlobal(ctx)
	return func(line string) {
		d, err := parseLine(ctx, line)
		if err != nil {
			g.Log.Errorf(errors.ErrorStack(err))
			// TODO continue when input is interactive (tty)
			return
		}
		err = g.Engine.ValidateExec(ctx, d)
		if err != nil {
			g.Log.Errorf(errors.ErrorStack(err))
		}
	}
}

func newTx(request mdb.Packet) engine.Doer {
	return engine.Func{Name: "mdb:" + request.Format(), F: func(ctx context.Context) error {
		g := state.GetGlobal(ctx)
		m, err := g.Mdb()
		if err != nil {
			return err
		}
		response := new(mdb.Packet)
		err = m.Tx(request, response)
		if err != nil {
			g.Log.Errorf(errors.ErrorStack(err))
		} else {
			g.Log.Infof("< %s", response.Format())
		}
		return err
	}}
}

func parseLine(ctx context.Context, line string) (engine.Doer, error) {
	g := state.GetGlobal(ctx)
	words := strings.Split(line, " ")
	empty := true
	for i, w := range words {
		wt := strings.TrimSpace(w)
		if wt != "" {
			empty = false
			words[i] = wt
		}
	}
	if empty {
		return engine.Nothing{}, nil
	}

	// pre-parse special commands
	loopn := uint(0)
	wordsRest := make([]string, 0, len(words))
	for _, word := range words {
		switch {
		case word == "help":
			g.Log.Infof(usage)
			return engine.Nothing{}, nil
		case strings.HasPrefix(word, "loop="):
			if loopn != 0 {
				return nil, errors.Errorf("multiple loop commands, expected at most one")
			}
			i, err := strconv.ParseUint(word[5:], 10, 32)
			if err != nil {
				return nil, errors.Annotatef(err, "word=%s", word)
			}
			loopn = uint(i)
		default:
			wordsRest = append(wordsRest, word)
		}
	}

	tx := engine.NewSeq("input:" + line)
	for _, word := range wordsRest {
		d, err := parseCommand(word)
		if d == nil && err == nil {
			g.Log.Fatalf("code error parseCommand word='%s' both doer and err are nil", word)
		}
		if err != nil {
			// TODO accumulate errors into list
			return nil, err
		}
		tx.Append(d)
	}

	if loopn != 0 {
		return engine.RepeatN{N: loopn, D: tx}, nil
	}
	return tx, nil
}

func parseCommand(word string) (engine.Doer, error) {
	switch {
	case word == "reset":
		return doBusReset, nil
	case word[0] == 's':
		i, err := strconv.ParseUint(word[1:], 10, 32)
		if err != nil {
			return nil, errors.Annotatef(err, "word=%s", word)
		}
		return engine.Sleep{Duration: time.Duration(i) * time.Millisecond}, nil
	case word[0] == '@':
		request, err := mdb.PacketFromHex(word[1:], true)
		if err != nil {
			return nil, err
		}
		return newTx(request), nil
	default:
		return nil, errors.Errorf("error: invalid command: '%s'", word)
	}
}
