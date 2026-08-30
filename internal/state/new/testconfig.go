package state_new

// WithConfig is a thin passthrough kept for existing call sites written
// before NewTestContext started loading defaultConfig.hcl as its base.
// extra no longer needs to be a complete, schema-valid HCL document — just
// the blocks a test wants to add or override — but code that already builds
// a bigger string (e.g. a locally-defined "minimal config" constant) still
// works fine passed through here, since decodeBody's per-fragment merge
// doesn't care how much or how little each fragment declares.
func WithConfig(extra string) string { return extra }
