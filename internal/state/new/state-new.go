// Sorry, workaround to import cycles.
package state_new

import (
	"context"
	"os"
	"testing"

	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
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
	// fs := state.NewMockFullReader(map[string]string{
	// 	"test-inline": confString,
	// })

	var log *log2.Log
	if os.Getenv("vender_test_log_stderr") == "1" {
		log = log2.NewStderr(log2.LOG_DEBUG) // useful with panics
	} else {
		log = log2.NewTest(t, log2.LOG_DEBUG)
	}
	log.SetFlags(log2.LTestFlags)
	ctx, g := NewContext(log, tele_api.NewStub())
	g.BuildVersion = buildVersion
	// g.MustInit(ctx, state.MustReadConfig_old(log, fs, "test-inline"))

	mdbus, mdbMock := mdb.NewMockBus(t)
	g.Hardware.Mdb.Bus = mdbus
	if _, err := g.Mdb(); err != nil {
		t.Fatal(errors.Trace(err))
	}
	ctx = context.WithValue(ctx, mdb.MockContextKey, mdbMock)

	return ctx, g
}
