// Helper for developing vender user interfaces
package ui

import (
	"context"
	"time"

	"github.com/AlexTransit/vender/cmd/vender/subcmd"
	"github.com/AlexTransit/vender/hardware"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/ui"
	"github.com/juju/errors"
)

var Mod = subcmd.Mod{Name: "ui", Main: Main}

func Main(ctx context.Context, args ...[]string) error {
	g := state.GetGlobal(ctx)
	g.Config.Engine.OnBoot = nil
	g.Config.Engine.OnMenuError = nil
	g.Config.Engine.Menu.Items = []*engine_config.MenuItem{
		{Code: "333", Name: "test item", XXX_Price: 5, Scenario: "sleep(3s)"},
	}
	g.MustInit(ctx, g.Config)
	g.Log.Debugf("config=%+v", g.Config)

	g.Log.Debugf("Init display")
	// textDisplay := g.MustTextDisplay()

	// helper to display all CLCD characters
	var bb [32]byte
	for b0 := 0; b0 < 256/len(bb); b0++ {
		for i := 0; i < len(bb); i++ {
			// bb[i] = byte(b0*len(bb) + i)
		}
		// textDisplay.SetLinesBytes(bb[:16], bb[16:])
		time.Sleep(1 * time.Second)
	}

	if err := hardware.InitMDBDevices(ctx); err != nil {
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
