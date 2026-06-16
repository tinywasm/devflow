# devflow — PLAN: Tool MCP `run_tests` (gotest)


> Estado: Borrador para revisión · Objetivo: expone `gotest` como una tool MCP única y
> mínima para que el LLM valide un proyecto (wasm + !wasm) con una sola llamada.
>
> ⚠️ Prescriptivo. Ver §6 (Invariantes) y §8 (Aceptación).

---

## 1. Contexto (verificado en código)

`gotest` es un método de librería en `Go`:

```go
// devflow/gotest.go:24
func (g *Go) Test(customArgs []string, skipRace bool, timeoutSec int, noCache bool, runAll bool) (string, error)
```

Ejecuta vet + stdlib + race + coverage y auto-detecta/ejecuta tests WASM bajo el capó — un
comando cubriendo wasm y !wasm (`devflow/docs/GOTEST.md`). Devuelve una línea resumen
(`vet ✅, race ✅, tests ✅, coverage: 85% (12.4s)`) y un error en caso de fallo.

El handler `Go` (`devflow/go_handler.go:19`) se construye con `NewGo(gitHandler)`
(`:58`), tiene `SetRootDir(path)` (`:82`) y `SetConsoleOutput(fn)` (`:105`). **app ya
posee uno**: `h.GoHandler *devflow.Go` (`app/handler.go:38`).

### Critical gotcha — working directory

`Test` runs in the **process CWD**, not `g.rootDir`. It calls `getModuleName(".")`
(`devflow/gotest.go:32`) and spawns `exec.Command("go", …)` / `RunCommand(…)` with no `Dir`
(e.g. `gotest.go:112,121,190,418,426,449,555`, plus `testCommand` at `:623`). The global MCP
daemon is long-running and runs the build watcher concurrently, so **`os.Chdir` is unsafe**
(process-global; would race the watcher/compiler). The tool must run tests on the project root
**without** chdir.

## 2. Objetivo

A single tool `run_tests`, minimal args, that runs the full suite on the active project root
and returns the summary. WASM handled automatically; the LLM provides nothing extra.

## 3. Diseño

### 3.1 Thread the working directory (prerequisite, race-free)

Make `gotest` honor `g.rootDir` instead of the CWD:

- `getModuleName(g.rootDir)` instead of `getModuleName(".")`.
- Set `cmd.Dir = g.rootDir` on every `exec.Command`/`testCommand`/`RunCommand` site used by
  `Test` (`gotest.go` and the `RunCommand` helpers it calls). `g.rootDir` defaults to `"."`,
  so existing CLI behavior (`devflow/cmd/gotest`) is unchanged.

> This is the correct fix vs `os.Chdir`; it keeps concurrent daemon work (watcher/compiler)
> unaffected. `SetRootDir` (`go_handler.go:82`) already exists to supply it.

### 3.2 The MCP provider

New file `devflow/gotest_mcp.go` (package `devflow`), importing `github.com/tinywasm/mcp`:

```go
// GoTestProvider exposes the gotest suite as a single MCP tool.
type GoTestProvider struct{ g *Go }

func NewGoTestProvider(g *Go) *GoTestProvider { return &GoTestProvider{g: g} }

// Tools implements mcp.ToolProvider.
func (p *GoTestProvider) Tools() []mcp.Tool { … }
```

One tool:

- name: `run_tests`
- Description: "Comprehensive Go test suite: runs vet, stdlib tests with race detection, exact
  coverage analysis, and auto-detected WASM tests. Full suite (no args) includes badges update
  and slow test detection. Fast path (with -run/flags) skips vet and badges. Intelligent caching
  by git state; cache disabled with custom flags."
- Resource: `"tests"`, Action: `'r'`
- schema (raw JSON string, daemon-style — no ormc codegen needed):

```json
{ "type": "object", "properties": {
  "run": { "type": "string", "description": "Optional: run only tests matching this name/pattern (e.g. TestFoo). Empty runs full suite: vet, race, coverage, WASM, badges. With custom flags uses fast path: vet and badges skipped, cache disabled." }
}}
```

Execute:

```go
run := string(unquote(mcp.ExtractJSONValue(argsBytes, "run")))  // via mcp.ExtractJSONValue
var summary string; var err error

// Full suite: vet + race + coverage + WASM + badges + cache
// Fast path: only go test + WASM auto-detect, no vet/badges/cache
if run == "" {
    summary, err = p.g.Test(nil, false, 0, false, false)        // full suite
} else {
    summary, err = p.g.Test([]string{"-run", run}, false, 0, false, false) // fast path
}

// Return summary (e.g. "vet ✅, race ✅, tests ✅, wasm ✅, coverage: 85% (12.4s)")
// on both success and failure; gotest embeds ✅/❌ and filtered output.
return mcp.Text(summary), nil
```

Console noise: `SetConsoleOutput` should route to the project logger (or a no-op) so streamed
test lines do not pollute stdout of the daemon; the returned `summary` is the LLM payload.

### 3.3 Registration (project-scoped, single source)

Per the app plan, aggregate via `buildProjectProviders` (`app/mcp_registry.go:67`) reusing
the existing handler:

```go
providers = append(providers, devflow.NewGoTestProvider(h.GoHandler))
```

`h.GoHandler.SetRootDir(h.RootDir)` must be ensured before use so tests run on the project root
(app wires this; `h.RootDir` is the active project root).

## 4. Steps

1. `gotest.go`: thread `g.rootDir` into `getModuleName` and all `exec`/`RunCommand` `Dir`s.
2. Create `devflow/gotest_mcp.go` with `GoTestProvider` + `run_tests`.
3. Route `SetConsoleOutput` to the injected logger; return `summary` as the result.
4. (app) register `devflow.NewGoTestProvider(h.GoHandler)` in `buildProjectProviders` and ensure
   `SetRootDir(h.RootDir)`.

## 5. Actualizaciones de documentación

- `devflow/docs/GOTEST.md`: add an "MCP tool" section documenting `run_tests` (no args = full
  suite; `run` = single test) and that wasm/!wasm run automatically.

## 6. Invariants / prohibitions

- **Do NOT** add more args than optional `run` (no timeout/race/coverage flags exposed).
- **Do NOT** use `os.Chdir` for the working directory (race with daemon). Use `cmd.Dir`.
- **Do NOT** change `Test`'s signature or its summary format.
- Keep CLI behavior identical (`g.rootDir` defaults to `"."`).

## 7. Tests (backing)

Add `test/gotest_mcp_test.go` (external test package, like the rest of `devflow/test/`):

1. **Full suite mapping:** `run==""` calls `Test(nil,false,0,false,false)` (inject a fake
   `GoTestCmdFn`, `gotest.go:20`, to avoid a real nested `go test`).
2. **Single test mapping:** `run="TestFoo"` calls `Test(["-run","TestFoo"], …)`.
3. **Working dir:** with `SetRootDir(tmpModule)`, assert the spawned command runs against that
   dir (assert `cmd.Dir` via the injected `GoTestCmdFn`).
4. **Result:** Execute returns the summary text on both success and failure.

Regression: existing `devflow/test/*` (gotest, gorelease, cli) pass unchanged; CLI default CWD
behavior preserved.

## 8. Acceptance (Definition of Done)

1. `run_tests` with no args runs the full suite on the active project root and returns the
   summary line.
2. `run_tests(run="TestFoo")` runs only that test.
3. WASM tests run automatically when present, with no extra LLM input.
4. Tests run on the project root **without** `os.Chdir` (daemon watcher unaffected).
5. `go build ./... && go test ./...` green (new + existing).

## 9. Open decision

- Daemon-level (uses the daemon's `lastPath`) vs project-scoped via proxy. Recommended:
  project-scoped through `buildProjectProviders` reusing `h.GoHandler` — visible with the
  active project and already root-configured.
