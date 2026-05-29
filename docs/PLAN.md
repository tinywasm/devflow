# PLAN: Fix `gotest` failing on the `devflow` repository

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> You are an external agent with **zero prior context** about this project. Everything
> you need is in this document. Do **not** run `gopush` or `codejob` — those are local
> developer tools handled outside your environment.

## 0. Prerequisite (run first)

The repository's test runner is a CLI named `gotest`. It is NOT globally available in
your isolated environment, so install it before doing anything else:

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

After your changes, you validate with:

```bash
gotest -no-cache
```

(`-no-cache` is mandatory for validation — see the cache note in §3.3, the bug is
intermittent and the cache can hide it.)

---

## 1. Symptom

Running the full test suite on the `devflow` module fails non-deterministically. Two
distinct failure modes are observed:

**Mode A — process killed / hang (`Terminated`):**
```
~/Dev/Project/tinywasm/devflow$ gotest
Terminated
```

**Mode B — package timeout panic:**
```
~/Dev/Project/tinywasm/devflow$ gotest -t 10
=== RUN   TestGoRelease_SingleCmd
panic: test timed out after 10s
        running tests:
                TestGoRelease_SingleCmd (1s)
...
github.com/tinywasm/devflow.(*Go).runFullTestSuite(...)  gotest.go
github.com/tinywasm/devflow.(*Go).Test(...)              gotest.go
github.com/tinywasm/devflow.(*Go).Push(...)              go_handler.go:188
github.com/tinywasm/devflow.(*Go).Release(...)           gorelease.go:21
github.com/tinywasm/devflow/test_test.TestGoRelease_SingleCmd(...)  test/gorelease_test.go
...
Tests failed: vet ✅, Test errors found in github.com/tinywasm/devflow ❌,
coverage: 45.5%, timeout: TestGoRelease_SingleCmd (exceeded 10s) ❌
```

A data race is also intermittently reported:
```
WARNING: DATA RACE
  ...TestGoPush_DependentOutput.func3()    test/go_handler_test.go (consoleLines append)
  ...(*Go).UpdateDependentModule()         go_handler.go
  ...(*Go).UpdateDependents.func1()        go_mod.go (goroutine)
--- FAIL: TestGoPush_DependentOutput
```

The same `gotest` runs cleanly and quickly on sibling modules (`tinywasm/dom`,
`tinywasm/time`) which finish in ~5–6s. Only `devflow` fails, because only `devflow`
tests its own publishing pipeline (`Push` / `Release`).

---

## 2. How `gotest` works (context you need)

`gotest` (the CLI under `cmd/gotest/`) calls the library function
`(*Go).Test(...)` → `(*Go).runFullTestSuite(...)` in `gotest.go`. For the full suite it
runs, in order:

1. `go vet ./...`
2. WASM test-file detection (`go list ...`)
3. **`go test -race -cover -coverpkg=./... -count=1 -timeout=<N>s ./...`** — the heavy step
4. README badge update

The per-package timeout defaults to **30s** (`-timeout=30s`). Go applies `-timeout` to the
**whole test binary of a package**, not per individual test. If the `devflow/test` package
takes longer than 30s in total, Go panics with `test timed out after 30s` and the run fails.

---

## 3. Root cause

There are **two independent bugs**, both rooted in the same design fact: several unit
tests exercise the real publishing pipeline, which **launches a real nested `go test`
subprocess** and **spawns parallel goroutines**.

### 3.1 Bug A — Real nested `go test` makes the package exceed its 30s timeout (Mode A & B)

`(*Go).Release(...)` (in `gorelease.go`) runs the full push workflow:

```go
// gorelease.go (HEAD)
func (g *Go) Release(message, tag string, gh *GitHub) error {
	cmds, err := g.listCmdDirs(g.rootDir)
	...
	// 2. Run full gopush workflow
	res, err := g.Push(message, tag, false, false, false, false, false, false, "..")
	//                              ^^^^^ skipTests = false  ← runs the FULL test suite
	...
}
```

`(*Go).Push(...)` then calls `Test`, which **really executes `go test -race`**:

```go
// go_handler.go:188 (HEAD)
if !skipTests {
	testSummary, err := g.Test([]string{}, skipRace, 0, false, false)
	...
}
```

```go
// gotest.go (HEAD) — runFullTestSuite spawns a REAL subprocess
testCtx, testCancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec+10)*time.Second)
defer testCancel()
testCmd := testCommand(testCtx, "go", testArgs...)   // ← real `go test`, NOT mockable
```

The unit tests `TestGoRelease_SingleCmd`, `TestGoRelease_MultipleCmd`,
`TestGoRelease_ExplicitTag`, and two sub-tests of `TestGoRelease_Errors`
(`Push fails`, `TmpDir cleaned up on error`) all call `Release(...)`. Each one therefore
launches a **real, race-instrumented `go test`** inside a temporary module. A cold
`-race` compile costs several seconds each. Combined with the other heavy tests
(`TestGoPush`, `TestGoPushFlags`, `TestGoUpdateModuleFail`), the `devflow/test` package
**creeps past the 30s package timeout** and Go panics (Mode B). When the slow run is
killed by the environment first, it shows as `Terminated` (Mode A).

The existing mock seam (`var ExecCommand = exec.Command`, used elsewhere in tests) does
**not** cover this path, because `runFullTestSuite` calls the unexported `testCommand`
(which wraps `exec.CommandContext`) directly — it cannot be intercepted from tests.

### 3.2 Bug B — Data race on `consoleLines` in `TestGoPush_DependentOutput`

`(*Go).UpdateDependents(...)` updates dependent modules **in parallel goroutines**:

```go
// go_mod.go (HEAD)
for _, depDir := range dependents {
	wg.Add(1)
	go func(dir string) {
		defer wg.Done()
		semaphore <- struct{}{}
		defer func() { <-semaphore }()
		g.UpdateDependentModule(dir, modulePath, version) // calls g.consoleOutput(...) inside
	}(depDir)
}
wg.Wait()
```

`UpdateDependentModule` calls `g.consoleOutput(...)` from each goroutine. The test installs
a `consoleOutput` callback that appends to a **plain slice with no synchronization**:

```go
// test/go_handler_test.go (HEAD) — TestGoPush_DependentOutput
var consoleLines []string
...
goHandler.SetConsoleOutput(func(s string) {
	consoleLines = append(consoleLines, s)   // ← concurrent append → DATA RACE
})
```

Under `-race` (which `gotest` always enables for the full suite) the concurrent
`append` is flagged as a data race and the test fails.

### 3.3 Why `gotest` (no args) sometimes shows nothing

`gotest` caches a successful full-suite result keyed on git state (`/tmp/gotest-cache/`).
When a race is not detected on a given run, the pass is cached and subsequent `gotest`
invocations return the cached result without re-running. The bug is intermittent, so it
can stay hidden until `gotest -no-cache` (or a flagged run such as `gotest -t 10`) forces
re-execution. **Always validate with `gotest -no-cache`.**

---

## 4. Required changes

Make the nested `go test` command injectable (so tests can stub it) and mock it in the
`Release`-exercising tests; then make the test's console callback concurrency-safe.

### Step 1 — `gotest.go`: expose an injectable command constructor

Add an exported package-level variable next to the existing `semverTagRe` declaration,
mirroring the existing `var ExecCommand = exec.Command` pattern already used in this
package:

```go
// gotest.go — near the top, after the imports / existing vars
// GoTestCmdFn creates the command used to run 'go test'. Override in tests to avoid
// launching a real nested `go test` subprocess (e.g. from Release→Push→Test).
var GoTestCmdFn = testCommand
```

Then replace the **two** non-WASM call sites that build the `go test` command so they go
through the variable instead of calling `testCommand` directly:

- In `runFullTestSuite`:
  ```go
  // BEFORE
  testCmd := testCommand(testCtx, "go", testArgs...)
  // AFTER
  testCmd := GoTestCmdFn(testCtx, "go", testArgs...)
  ```
- In `runCustomTests`:
  ```go
  // BEFORE
  testCmd := testCommand(customCtx, "go", testArgs...)
  // AFTER
  testCmd := GoTestCmdFn(customCtx, "go", testArgs...)
  ```

Do **not** change the WASM-path call sites (`wasmCmd := testCommand(wasmCtx, ...)`) — those
are unrelated and `devflow` has no WASM tests.

`testCommand` keeps its signature `func(ctx context.Context, name string, args ...string) *exec.Cmd`,
so `GoTestCmdFn` has that type automatically.

### Step 2 — `test/gorelease_test.go`: stub the nested `go test`

Add a helper that swaps `GoTestCmdFn` for a fast stub that returns a successful
`go test`-like output, and restores it afterwards. Use only the standard library
(`os/exec`, `context`, `testing`):

```go
// add imports: "context", "os/exec"

// mockGoTest replaces GoTestCmdFn so Release→Push→Test does not spawn a real
// `go test` subprocess. Returns a restore function.
func mockGoTest(t *testing.T) func() {
	t.Helper()
	orig := devflow.GoTestCmdFn
	devflow.GoTestCmdFn = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("echo", "ok  testmodule\t0.1s\ncoverage: 100% of statements in ./...")
	}
	return func() { devflow.GoTestCmdFn = orig }
}
```

Call it as the **first line** of every test that reaches `Release(...)`:

- `TestGoRelease_SingleCmd`        → `defer mockGoTest(t)()`
- `TestGoRelease_MultipleCmd`      → `defer mockGoTest(t)()`
- `TestGoRelease_ExplicitTag`      → `defer mockGoTest(t)()`
- `TestGoRelease_Errors` sub-test `"Push fails"`              → `defer mockGoTest(t)()`
- `TestGoRelease_Errors` sub-test `"TmpDir cleaned up on error"` → `defer mockGoTest(t)()`

(The `"No cmd dir"` sub-test returns before `Push`, so it does not need the stub.)
Do not change any assertions — the tests must still verify the same behavior (gh args,
asset names, tag pushed, tmp dir cleanup).

### Step 3 — `test/go_handler_test.go`: make `consoleLines` concurrency-safe

In `TestGoPush_DependentOutput`, guard the slice with a mutex (standard library `sync`):

```go
// add "sync" to the import block

var consoleLines []string
var consoleMu sync.Mutex
...
goHandler.SetConsoleOutput(func(s string) {
	consoleMu.Lock()
	consoleLines = append(consoleLines, s)
	consoleMu.Unlock()
})
```

If later assertions read `consoleLines`, they run after `Push` returns (all goroutines
joined via `wg.Wait()`), so reads need no lock — only the concurrent appends do.

---

## 5. Constraints (must follow)

- **Standard library only** in tests. Do NOT add `testify`, `gomega`, or any assertion
  library. Use `testing`, `os/exec`, `context`, `sync`, `reflect`.
- **Tests must not perform real I/O / subprocesses.** The whole point of Step 2 is to
  remove the real nested `go test`. Do not introduce new real `go`/`git` subprocess calls.
- **No hardcoded duplicated strings in library logic.** If you need a repeated literal,
  use a named constant. (The `echo`-based stub in test code is fine as-is.)
- **Thin `cmd/`.** Do not move logic into `cmd/gotest/main.go`; all behavior stays in the
  library package and is exercised through exported symbols.
- **Do not change** `gotest`'s default timeout, the `-race`/`-coverpkg` flags, or the
  badge logic. The suite must pass within the existing 30s default once the nested real
  `go test` runs are stubbed.
- Keep `GoTestCmdFn` defaulting to the real `testCommand` so production behavior is
  unchanged; only tests override it.

---

## 6. Verification

Run from the module root (`devflow/`, next to `go.mod`):

```bash
go build ./...
gotest -no-cache
```

Expected result:

- No `DATA RACE` warning.
- No `panic: test timed out`.
- No `Terminated`.
- A clean summary similar to:
  `vet ✅, race ✅, tests ✅, coverage: <pct>% (<~18s>)`

Sanity targets (with `-race`, the suite must finish well under 30s):

- `TestGoRelease_SingleCmd` drops from ~8s to well under 1s (no real nested `go test`).
- Full `devflow/test` package finishes in roughly ~15–20s.

Also confirm the individual fixed tests pass under the race detector:

```bash
go test ./test/ -race -run 'TestGoRelease|TestGoPush_DependentOutput' -v
```

---

## 7. Stages

| Stage | File | Change | Done when |
|-------|------|--------|-----------|
| 1 | `gotest.go` | Add `var GoTestCmdFn = testCommand`; route the two non-WASM `testCommand(...)` call sites (`runFullTestSuite`, `runCustomTests`) through it | `go build ./...` passes; WASM call sites untouched |
| 2 | `test/gorelease_test.go` | Add `mockGoTest` helper (imports `context`, `os/exec`); `defer mockGoTest(t)()` in the 5 `Release`-calling tests/sub-tests | `go test ./test/ -race -run TestGoRelease -v` passes fast |
| 3 | `test/go_handler_test.go` | Add `sync` import; guard `consoleLines` append with `sync.Mutex` in `TestGoPush_DependentOutput` | `go test ./test/ -race -run TestGoPush_DependentOutput -v` passes, no race |
| 4 | — | Full validation | `gotest -no-cache` is green, no race / timeout / Terminated, suite < 30s |
