# PLAN: gotest Per-Test Stall Watchdog

Status: **PROPOSED** — sub-plan 1 of [PLAN.md](PLAN.md). Read the orchestrator's Development Rules first; execute this plan before [GOPUSH_SELFDEP_PLAN.md](GOPUSH_SELFDEP_PLAN.md) (gopush's test gate depends on it).

## Plan-specific rules

In addition to the orchestrator's shared Development Rules:

- New behavior goes in its own small file (`watchdog.go`), not appended to `gotest.go`.
- Only the *semantics* of `-t` are refined (see below); the flag itself keeps working and the change is documented in `docs/GOTEST.md`.

## Problem

`gotest` injects `-timeout=<t>s` (default 30s) into `go test`. Go applies that timeout **per test binary (package), cumulatively** — not per test. When a package holds many tests, the budget is spent by the *sum* of all tests, and Go kills whichever test happens to be running when the alarm fires, even if it just started.

Real failure (devflow itself):

```
panic: test timed out after 30s
        running tests:
                TestGoNewCreateLocalOnly (0s)   ← blamed, but only 0s elapsed
...
⚠️ slow: TestGoUpdateModuleFail (4.2s)
Tests failed: ... timeout: TestGoNewCreateLocalOnly (exceeded 30s) ❌
```

`TestGoNewCreateLocalOnly` was not hung — the suite's cumulative time crossed 30s. Raising `-t` "fixes" it until the suite grows again: an infinite arms race. The timeout exists to catch **hung tests fast**, not to cap total suite duration.

## Design: inactivity watchdog

Change what `-t N` means: from *"package budget"* to *"maximum time a single test may run without finishing"*. A progressing suite can take as long as it needs; a genuinely stuck test is still killed after N seconds.

Mechanism (see [diagram](diagrams/GOTEST_WATCHDOG.md)):

1. `gotest` already streams `go test -v` output through `paramWriter` → `ConsoleFilter`. A new `Watchdog` type taps the same stream.
2. The watchdog parses test lifecycle events: `=== RUN`, `=== PAUSE`, `=== CONT`, `--- PASS/FAIL/SKIP`, `ok \t`, `FAIL\t`. It maintains a set of currently-running tests with their start (or resume) timestamps.
3. A ticker goroutine checks: if any running test has been active longer than `timeoutSec`, cancel the command context (kill the process) and record that test as the culprit.
4. The Go-native `-timeout` is still injected as a **backstop**, at `timeoutSec × 10` (so Go's own panic with stack traces still fires if the watchdog itself is bypassed, e.g. output redirected).
5. Reporting distinguishes the two cases:
   - Watchdog kill: `❌ timeout: TestX stalled >30s (no progress)`
   - Backstop kill: `❌ timeout: package exceeded 300s total` — plus the slow-test list already produced by `FindSlowestTest`.

Notes:

- `runCustomTests` (fast path) does not currently pass `-v`; the watchdog needs lifecycle events, so `-v` is injected there too (`ConsoleFilter` already suppresses the noise).
- Parallel tests: `=== PAUSE` removes a test from the active set; `=== CONT` re-adds it with a fresh timestamp, so paused tests are never blamed.
- The watchdog must be created per command (native, submodules, WASM) — same places `ConsoleFilter` is instantiated today.

## Steps

### Phase 1 — Watchdog core (TDD)

1. `test/watchdog_test.go`: failing tests first —
   - a stream of many fast tests whose *total* exceeds the limit does **not** trigger a kill (the original bug);
   - a single test with no completion event within the limit **does** trigger, and the culprit name is exact;
   - `PAUSE`/`CONT` sequences do not blame paused tests;
   - fragmented lines (no trailing `\n`) are handled, same contract as `ConsoleFilter.Add`.
   - Use short limits (~50–200ms) and an injectable clock/`onKill` callback — no real processes.
2. `watchdog.go`: implement `Watchdog` (`Add(string)`, `Start(ctx cancel)`, `Stop()`, `Culprits() []string`).

### Phase 2 — Wiring

3. In `runFullTestSuite` and `runCustomTests`: feed the watchdog from the existing `paramWriter`, replace injected `-timeout=<t>s` with `-timeout=<t*10>s` backstop, inject `-v` in the custom path, cancel the command context on watchdog fire. Cover with a test using `GoTestCmdFn` (fake command emitting a scripted event stream).

### Phase 3 — Reporting

4. Distinguish `stalled` vs `backstop` messages in the summary; keep `FindTimedOutTests` as fallback for backstop/external kills. Update existing timeout tests.

### Phase 4 — devflow's own suite hygiene

5. Tag the heavy tests that do real git/keyring work (`TestGoNewCreateLocalOnly`, gopush/gonew integration-style tests) with `//go:build integration` so the default `gotest` run stays fast; they run via `gotest -all`. This is independent of the watchdog but removes today's noise.

### Phase 5 — Docs & release

6. Update `docs/GOTEST.md` "Timeout" section (per-test stall semantics, backstop, `-all`). Link this plan and the diagram from `README.md`. Publish with `gopush`.

## Test strategy summary

| Case | Expected |
|---|---|
| 40 tests × 1s each, `-t 30` | Suite passes (total 40s > 30s is fine) |
| One test silent for 31s, `-t 30` | Killed at ~30s, blamed by name, `stalled` message |
| Parallel: paused 60s, active progressing | No kill |
| Backstop reached (watchdog bypassed) | `package exceeded Ns total` + slowest tests listed |
