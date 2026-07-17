package devflow

import "time"

// tinygoExecArg runs the WASM suite through TinyGo instead of the Go toolchain.
// All the TinyGo logic (locating the toolchain, rebuilding the package, serving
// TinyGo's wasm_exec.js) lives in wasmbrowsertest — devflow only passes the flag,
// so it never takes a dependency on the TinyGo installer.
const tinygoExecArg = "wasmbrowsertest -tinygo"

// tinygoTimeout is the ceiling for a TinyGo WASM run. TinyGo compiles through
// LLVM and is roughly two orders of magnitude slower than `go build`: a small
// crypto package takes ~150s. The regular per-package timeout would kill it
// before the browser ever starts.
const tinygoTimeout = 10 * time.Minute

// UseTinygo makes the WASM suite compile with TinyGo.
//
// It is opt-in because it is slow. It is also the only run that proves anything
// about TinyGo compatibility: the Go js/wasm backend supports the full stdlib,
// so the default WASM suite stays green on packages TinyGo cannot build.
func (g *Go) UseTinygo(enabled bool) { g.useTinygo = enabled }

func (g *Go) wasmExecArg() string {
	if g.useTinygo {
		return tinygoExecArg
	}
	return "wasmbrowsertest"
}

func (g *Go) wasmTimeout(timeoutSec int) time.Duration {
	if g.useTinygo {
		return tinygoTimeout
	}
	return time.Duration(timeoutSec+10) * time.Second
}
