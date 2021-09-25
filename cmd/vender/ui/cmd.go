// Helper for developing vender user interfaces
package ui

import (
	"context"
	"time"

	"github.com/juju/errors"
	"github.com/temoto/vender/cmd/vender/subcmd"
	"github.com/temoto/vender/hardware"
	engine_config "github.com/temoto/vender/internal/engine/config"
	"github.com/temoto/vender/internal/money"
	"github.com/temoto/vender/internal/state"
	"github.com/temoto/vender/internal/ui"
)

var Mod = subcmd.Mod{Name: "ui", Main: Main}

func Main(ctx context.Context, config *state.Config) error {
	g := state.GetGlobal(ctx)
	config.Engine.OnBoot = nil
	config.Engine.OnMenuError = nil
	config.Engine.Menu.Items = []*engine_config.MenuItem{
		&engine_config.MenuItem{Code: "333", Name: "test item", XXX_Price: 5, Scenario: "sleep(3s)"},
	}
	g.MustInit(ctx, config)
	g.Log.Debugf("config=%+v", g.Config)

	g.Log.Debugf("Init display")
	textDisplay := g.MustTextDisplay()

	// helper to display all CLCD characters
	var bb [32]byte
	for b0 := 0; b0 < 256/len(bb); b0++ {
		for i := 0; i < len(bb); i++ {
			bb[i] = byte(b0*len(bb) + i)
		}
		textDisplay.SetLinesBytes(bb[:16], bb[16:])
		time.Sleep(1 * time.Second)
	}

	if err := hardware.Enum(ctx); err != nil {
		err = errors.Annotate(err, "hardware enum")
		return err
	}

	moneysys := new(money.MoneySystem)
	if err := moneysys.Start(ctx); err != nil {
		err = errors.Annotate(err, "money system Start()")
		return err
	}

	g.Log.Debugf("init complete, enter main loop")
	ui := ui.UI{}
	if err := ui.Init(ctx); err != nil {
		err = errors.Annotate(err, "ui Init()")
		return err
	}
	ui.Loop(ctx)
	return nil
}
