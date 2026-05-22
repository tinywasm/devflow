> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan: gotest — Fix FindTimedOutTests falsely reporting skipped WASM tests as timeout

## Bug

`FindTimedOutTests` in `gotest.go` has a fallback path (line ~957) that finds the last
`=== RUN` line without a matching `--- PASS:` or `--- FAIL:`. It does **not** clear
`lastRun` when it sees `--- SKIP:`, so a skipped test is falsely reported as timed out.

### Reproduction

In `tinywasm/jsvalue`, `TestAwaitRequest_success` only called `t.Skip(...)`. Output:

```
=== RUN   TestAwaitRequest_success
    async_wasm_test.go:41: covered by tinywasm/indexdb integration tests
--- SKIP: TestAwaitRequest_success (0.00s)
vet ✅, wasm ✅, coverage: 83.5%, timeout: TestAwaitRequest_success (exceeded 30s) ❌
```

The test ran in 0.00s and was skipped, yet gotest reported it as timeout.

## Root Cause

`FindTimedOutTests` fallback loop (`gotest.go`):

```go
if strings.Contains(line, "--- PASS:") || strings.Contains(line, "--- FAIL:") {
    lastRun = ""
}
```

Missing: `--- SKIP:` does not clear `lastRun`.

## Fix

Add `--- SKIP:` to the condition that clears `lastRun`:

```go
if strings.Contains(line, "--- PASS:") || strings.Contains(line, "--- FAIL:") || strings.Contains(line, "--- SKIP:") {
    lastRun = ""
}
```

## Test to Add

Add a new case to `TestFindTimedOutTests` in `devflow/test/gotest_test.go`:

```go
{
    name: "Skipped test must not be reported as timeout",
    output: `=== RUN   TestAwaitRequest_success
    async_wasm_test.go:41: covered by tinywasm/indexdb integration tests
--- SKIP: TestAwaitRequest_success (0.00s)`,
    expected: nil,
},
```

This case reproduces the exact output that triggered the bug in `tinywasm/jsvalue`.

## Stages

| # | Archivo | Acción |
|---|---|---|
| 1 | `devflow/gotest.go` | En `FindTimedOutTests` fallback loop: agregar `\|\| strings.Contains(line, "--- SKIP:")` a la condición que limpia `lastRun` |
| 2 | `devflow/test/gotest_test.go` | Agregar caso `"Skipped test must not be reported as timeout"` a `TestFindTimedOutTests` |

## Verification

```bash
gotest
```

`TestFindTimedOutTests` debe pasar. Sin regresiones.
