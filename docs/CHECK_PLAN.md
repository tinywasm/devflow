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

(`-no-cache` is mandatory for validation — see the cache note in §3.2, the bug is
intermittent and the cache can hide it.)

---

## 1. Symptom

Running the full test suite on the `devflow` module fails non-deterministically under
`-race`. The observed failure mode:

**Data race on `consoleLines`:**
```
WARNING: DATA RACE
  ...TestGoPush_DependentOutput.func3()    test/go_handler_test.go (consoleLines append)
  ...(*Go).UpdateDependentModule()         go_handler.go
  ...(*Go).UpdateDependents.func1()        go_mod.go (goroutine)
--- FAIL: TestGoPush_DependentOutput
```

The same `gotest` runs cleanly and quickly on sibling modules (`tinywasm/dom`,
`tinywasm/time`). Only `devflow` fails here because only `devflow` tests its own
`UpdateDependents` flow which spawns goroutines.

---

## 2. How `gotest` works (context you need)

`gotest` (the CLI under `cmd/gotest/`) calls the library function
`(*Go).Test(...)` → `(*Go).runFullTestSuite(...)` in `gotest.go`. For the full suite it
runs, in order:

1. `go vet ./...`
2. WASM test-file detection (`go list ...`)
3. **`go test -race -cover -coverpkg=./... -count=1 -timeout=<N>s ./...`** — the heavy step
4. README badge update

The per-package timeout defaults to **30s** (`-timeout=30s`). The `-race` flag is always
enabled in the full suite.

---

## 3. Root cause

### 3.1 Bug — Data race on `consoleLines` in `TestGoPush_DependentOutput`

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

### 3.2 Why `gotest` (no args) sometimes shows nothing

`gotest` caches a successful full-suite result keyed on git state (`/tmp/gotest-cache/`).
When a race is not detected on a given run, the pass is cached and subsequent `gotest`
invocations return the cached result without re-running. The bug is intermittent, so it
can stay hidden until `gotest -no-cache` (or a flagged run such as `gotest -t 10`) forces
re-execution. **Always validate with `gotest -no-cache`.**

---

## 4. Required change

### Step 1 — `test/go_handler_test.go`: make `consoleLines` concurrency-safe

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
- **Tests must not perform real I/O / subprocesses.** Do not introduce new real `go`/`git`
  subprocess calls.
- **No hardcoded duplicated strings in library logic.** If you need a repeated literal,
  use a named constant.
- **Thin `cmd/`.** Do not move logic into `cmd/gotest/main.go`; all behavior stays in the
  library package and is exercised through exported symbols.
- **Do not change** `gotest`'s default timeout, the `-race`/`-coverpkg` flags, or the
  badge logic.

---

## 6. Verification

Run from the module root (`devflow/`, next to `go.mod`):

```bash
go build ./...
gotest -no-cache
```

Expected result:

- No `DATA RACE` warning.
- A clean summary similar to:
  `vet ✅, race ✅, tests ✅, coverage: <pct>%`

Also confirm the individual fixed test passes under the race detector:

```bash
go test ./test/ -race -run 'TestGoPush_DependentOutput' -v
```

---

## 7. Stages

| Stage | File | Change | Done when |
|-------|------|--------|-----------|
| 1 | `test/go_handler_test.go` | Add `sync` import; guard `consoleLines` append with `sync.Mutex` in `TestGoPush_DependentOutput` | `go test ./test/ -race -run TestGoPush_DependentOutput -v` passes, no race |
| 2 | — | Full validation | `gotest -no-cache` is green, no race warning, suite passes |
