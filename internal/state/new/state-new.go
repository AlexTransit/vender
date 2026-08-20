// Sorry, workaround to import cycles.
package state_new

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AlexTransit/vender/hardware/mdb"
	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/log2"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
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

// defaultConfigPath locates <repo root>/defaultConfig.hcl relative to this
// source file's own location, so it resolves correctly no matter which
// package directory `go test` runs from (its working directory is always
// the package under test, never the repo root).
//
// defaultConfigPath assumes this file lives at internal/state/new/ — three
// directories below the repo root — same as config_global.WriteConfigToFile
// writes it, from the repo root, via `go run ./cmd/... write.config` or
// equivalent.
func defaultConfigPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "defaultConfig.hcl")
}

// decodeBody parses and decodes one HCL fragment into cfg, using the exact
// merge semantics config_global.ReadConfig uses for multi-file configs:
// gohcl.DecodeBody only requires the blocks THIS fragment declares, and only
// checks for duplicates within itself — fields it doesn't mention keep
// whatever a previous decodeBody call already set. That's what lets tests
// layer a small, focused HCL snippet on top of the full defaultConfig.hcl
// base without re-declaring every required block.
func decodeBody(t testing.TB, cfg *config_global.Config, filename string, src []byte) {
	t.Helper()
	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		t.Fatalf("parse %s: %s", filename, diags)
	}
	if diags := gohcl.DecodeBody(file.Body, nil, cfg); diags.HasErrors() {
		t.Fatalf("decode %s: %s", filename, diags)
	}
	config_global.ProcessConfig(cfg)
}

// NewTestContext builds a Global/context for tests. The config always starts
// from the repo's defaultConfig.hcl (schema-complete by construction — it's
// generated from the same Config struct these tests decode into), then
// layers confString on top if given. confString only needs to contain the
// blocks a particular test cares about (e.g. `inventory { stock "x" {...} }`)
// — it does not need to be a complete, standalone config.
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

	cfg := config_global.NewConfig()
	path := defaultConfigPath()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (tests need defaultConfig.hcl at the repo root — "+
			"regenerate it with config_global.WriteConfigToFile if it's missing or stale)", path, err)
	}
	decodeBody(t, cfg, path, src)

	if confString != "" {
		decodeBody(t, cfg, "test-inline.hcl", []byte(confString))
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
