> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan: gopush — Skip dependent auto-push when CODEJOB session is active

## Problem

When `gopush` publishes a module and auto-updates its dependents, it runs
`UpdateDependentModule` for each dependent in parallel. If a dependent has an
active Jules session (`CODEJOB=driver:sessionID` in its `.env`), the auto-push
modifies `go.mod` while Jules is working on a branch that also touches `go.mod`
— causing a merge conflict that breaks the codejob loop.

**Root cause:** `UpdateDependentModule` in `go_handler.go` only checks for
`replace` directives before deciding to skip. It has no awareness of active
codejob sessions.

## State model

The real state of a codejob session lives in the dependent's `.env`:

| `.env` key | Meaning | Conflict risk |
|---|---|---|
| `CODEJOB=driver:sessionID` | Jules is actively working — branch being written | ✅ HIGH — skip push |
| `CODEJOB_PR=https://...` | PR open, waiting review — Jules done writing | ❌ none — push is safe |
| neither | No active session | ❌ none — push normally |

Only `CODEJOB` (not `CODEJOB_PR`) signals an active Jules session where `go.mod`
may be concurrently modified.

## Decision

**Read `.env` of the dependent and check `CODEJOB` before pushing.**
If `CODEJOB` is set and non-empty → skip push with `⏭ skip (codejob active)`,
same behaviour as `HasOtherReplaces`.

The `go.mod` bump (`go get` + `go mod tidy`) still runs so the dependency stays
current on disk. Only the **push** is skipped.

**Why `.env` / `CODEJOB`, not `PLAN.md` / `CHECK_PLAN.md`:**
- `CODEJOB=` is the authoritative runtime state — set by `codejob` when it
  dispatches to Jules, cleared when the loop closes.
- `PLAN.md` is pre-dispatch intent (no branch yet) — safe to push over.
- `CHECK_PLAN.md` means Jules is done writing — safe to push over.
- Reading `.env` is already done by `devflow/env.go`; no new primitives needed.

## Changes

### `go_handler.go` — `UpdateDependentModule`

Use the existing constants `EnvKeyCodejob` from `codejob.go`.

Add an exported helper (reusable, testable):

```go
// HasActiveCodejobSession reports whether dir has a Jules session in progress.
// It reads CODEJOB from the dir's .env file.
// Only an active session (CODEJOB set) blocks dependent auto-push;
// CODEJOB_PR (PR open, Jules done) does not.
func HasActiveCodejobSession(dir string) bool {
    e := NewEnvFile(filepath.Join(dir, ".env"))
    val, ok := e.Get(EnvKeyCodejob)
    return ok && val != ""
}
```

In `UpdateDependentModule`, after step 5 (`go mod tidy`) and **before** the
`HasOtherReplaces` check, add:

```go
if HasActiveCodejobSession(depDir) {
    g.consoleOutput(fmt.Sprintf("📦 %s → skip (codejob active) ⏭", depName))
    return "updated (codejob active, push skipped)", nil
}
```

### `devflow/test/go_handler_test.go` — new test

```go
func TestHasActiveCodejobSession(t *testing.T) {
    dir := t.TempDir()
    envPath := filepath.Join(dir, ".env")

    // No .env → not active
    if devflow.HasActiveCodejobSession(dir) {
        t.Fatal("expected false when .env missing")
    }

    // CODEJOB set → active
    os.WriteFile(envPath, []byte("CODEJOB=jules:12345\n"), 0644)
    if !devflow.HasActiveCodejobSession(dir) {
        t.Fatal("expected true when CODEJOB is set")
    }

    // Only CODEJOB_PR → NOT active (Jules done writing)
    os.WriteFile(envPath, []byte("CODEJOB_PR=https://github.com/org/repo/pull/1\n"), 0644)
    if devflow.HasActiveCodejobSession(dir) {
        t.Fatal("expected false when only CODEJOB_PR is set")
    }

    // Empty CODEJOB → not active
    os.WriteFile(envPath, []byte("CODEJOB=\n"), 0644)
    if devflow.HasActiveCodejobSession(dir) {
        t.Fatal("expected false when CODEJOB is empty")
    }
}
```

## Diagram update

Update `docs/diagrams/GOPUSH_FLOW.md` — add decision node after `go mod tidy`:

```
L2[go get + go mod tidy]
L2 --> LC{.env CODEJOB set?}
LC -- Yes --> LS[Print: 📦 dep ⏭ skip (codejob active)]
LC -- No --> L3{Other replaces?}
```

## Stages Summary

| # | Archivo | Acción |
|---|---|---|
| 1 | `devflow/go_handler.go` | Agregar `HasActiveCodejobSession(dir string) bool`; llamarla en `UpdateDependentModule` antes de `HasOtherReplaces` |
| 2 | `devflow/test/go_handler_test.go` | Agregar `TestHasActiveCodejobSession` |
| 3 | `devflow/docs/diagrams/GOPUSH_FLOW.md` | Agregar nodo `LC` al diagrama de dependientes |

## Verification

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
gotest
```

Sin regresiones. `TestHasActiveCodejobSession` debe pasar.
