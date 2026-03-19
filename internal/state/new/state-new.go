// Sorry, workaround to import cycles.
package state_new

import (
	"context"
	"os"
	"testing"

	"github.com/AlexTransit/vender/hardware/mdb"
	config_global "github.com/AlexTransit/vender/internal/config"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	menu_config "github.com/AlexTransit/vender/internal/menu/menu_config"
	"github.com/AlexTransit/vender/internal/state"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

func NewContext(log *log2.Log, teler tele_api.Teler) (context.Context, *state.Global) {
	if log == nil {
		panic("code error NewContext() log=nil")
	}

	g := &state.Global{
		Alive:     alive.NewAlive(),
		Engine:    engine.NewEngine(log),
		Inventory: new(inventory.Inventory),
		Log:       log,
		Tele:      teler,
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, log2.ContextKey, log)
	ctx = context.WithValue(ctx, engine.ContextKey, g.Engine)
	ctx = context.WithValue(ctx, state.ContextKey, g)

	return ctx, g
}

func NewTestContext(t testing.TB, buildVersion string, confString string) (context.Context, *state.Global) {
	var log *log2.Log
	if os.Getenv("vender_test_log_stderr") == "1" {
		log = log2.NewStderr(log2.LOG_DEBUG) // useful with panics
	} else {
		log = log2.NewTest(t, log2.LOG_DEBUG)
	}
	log.SetFlags(log2.LTestFlags)
	ctx, g := NewContext(log, tele_api.NewStub())
	g.BuildVersion = buildVersion
	cfg := testConfig()
	if confString != "" {
		if err := hclsimple.Decode("test-inline.hcl", []byte(confString), nil, cfg); err != nil {
			t.Fatal(errors.Trace(err))
		}
	}
	g.Config = cfg

	mdbus, mdbMock := mdb.NewMockBus(t)
	g.Hardware.Mdb.Bus = mdbus
	if _, err := g.Mdb(); err != nil {
		t.Fatal(errors.Trace(err))
	}
	ctx = context.WithValue(ctx, mdb.MockContextKey, mdbMock)

	return ctx, g
}

func testConfig() *config_global.Config {
	cfg := config_global.VMC
	cfg.Inventory.XXX_Stocks = copyStockMap(config_global.VMC.Inventory.XXX_Stocks)
	cfg.Inventory.XXX_Ingredient = copyIngredientMap(config_global.VMC.Inventory.XXX_Ingredient)
	cfg.Hardware.EvendDevices = copyDeviceMap(config_global.VMC.Hardware.EvendDevices)
	cfg.UI_config.Service.Tests = copyUITestMap(config_global.VMC.UI_config.Service.Tests)
	cfg.Engine.Aliases = copyAliasMap(config_global.VMC.Engine.Aliases)
	cfg.Engine.Menu.Items = copyMenuItemMap(config_global.VMC.Engine.Menu.Items)
	return &cfg
}

func copyStockMap(in map[string]inventory.Stock) map[string]inventory.Stock {
	out := make(map[string]inventory.Stock, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyIngredientMap(in map[string]inventory.Ingredient) map[string]inventory.Ingredient {
	out := make(map[string]inventory.Ingredient, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyDeviceMap(in map[string]config_global.DeviceConfig) map[string]config_global.DeviceConfig {
	out := make(map[string]config_global.DeviceConfig, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyUITestMap(in map[string]ui_config.TestsStruct) map[string]ui_config.TestsStruct {
	out := make(map[string]ui_config.TestsStruct, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyAliasMap(in map[string]engine_config.Alias) map[string]engine_config.Alias {
	out := make(map[string]engine_config.Alias, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyMenuItemMap(in map[string]menu_config.MenuItem) map[string]menu_config.MenuItem {
	out := make(map[string]menu_config.MenuItem, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
