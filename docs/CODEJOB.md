# CodeJob

`CodeJob` is a chain-of-responsibility orchestrator that sends a coding task (defined in an `PLAN.md` file) to a sequence of external AI agent drivers. It tries each driver in priority order and falls back automatically on failure.

## Architecture

```
CodeJob.Send(path)
  └─ autoDetectTitle() → "owner/repo"
  └─ drivers[0].Send(prompt, title) → success → return ✅
  └─ drivers[0].Send(prompt, title) → error   → try next
  └─ drivers[1].Send(prompt, title) → success → return ✅
  └─ all fail                       → return aggregated error ❌
```

Diagrams:
- [CODEJOB_INIT_FLOW.md](codejob/diagrams/CODEJOB_INIT_FLOW.md) — one-time setup wizard
- [CODEJOB_DISPATCH_FLOW.md](codejob/diagrams/CODEJOB_DISPATCH_FLOW.md) — local dispatch flow
- [GOPUSH_CODEJOB_FLOW.md](codejob/diagrams/GOPUSH_CODEJOB_FLOW.md) — integrated flow via gopush

## Key Types

### `CodeJobDriver` interface
```go
type CodeJobDriver interface {
    Name() string
    SetLog(fn func(...any))
    // prompt: "Execute the implementation plan described in docs/PLAN.md"
    // title:  "owner/repo" derived by CodeJob via autoDetectTitle()
    Send(prompt, title string) (string, error)
}
```

### `CodeJob` orchestrator
```go
job := devflow.NewCodeJob(drivers ...CodeJobDriver)
job.SetLog(log.Println)
result, err := job.Send("docs/PLAN.md")
```

## Setup (one-time per repo)

```bash
codejob init
```

The interactive wizard prompts for your Jules API key (from [jules.google.com](https://jules.google.com)) and stores it in the system keyring. That's it — no CI setup, no YAML files.

## Authentication & Auto-Detection

Jules API key is managed via the system keyring (`github.com/zalando/go-keyring`):
- **After `codejob init`**: reads silently from keyring — no env vars required in local use

GitHub repo and branch are auto-detected when not provided:
- `SourceID` — via `gh repo view --json owner,name`
- `StartBranch` — via `git branch --show-current`

## Adding a New Driver

Implement `CodeJobDriver` and pass it to `NewCodeJob`:

```go
type MyDriver struct{}

func (d *MyDriver) Name() string                                { return "MyAgent" }
func (d *MyDriver) SetLog(fn func(...any))                      {}
func (d *MyDriver) Send(prompt, title string) (string, error)   { /* ... */ }

job := devflow.NewCodeJob(devflow.NewJulesDriver(devflow.JulesConfig{}), &MyDriver{})
```

## Usage

### CLI
```bash
go install github.com/tinywasm/devflow/cmd/codejob@latest

# One-time setup: saves Jules API key to system keyring
codejob init

# Dispatch (default path: docs/PLAN.md)
codejob

# Explicit path
codejob path/to/PLAN.md

# Close the loop after reviewing PR
# If docs/PLAN.md exists (e.g. pulled from main), this automatically dispatches the next job.
codejob done
codejob done v0.3.0  # merge and publish with explicit tag
```

### Go Library
```go
// Zero-config: API key from keyring, repo/branch auto-detected via gh/git
job := devflow.NewCodeJob(devflow.NewJulesDriver(devflow.JulesConfig{}))
result, err := job.Send("docs/PLAN.md")

// Explicit config (override any field)
cfg := devflow.JulesConfig{
    APIKey:      "key",                        // optional: defaults to keyring
    SourceID:    "sources/github/user/repo",  // optional: auto-detected via gh
    StartBranch: "main",                       // optional: auto-detected via git
}
job := devflow.NewCodeJob(devflow.NewJulesDriver(cfg))
```

## Integrated Flow (via push / gopush)

CodeJob dispatch lives inside `Git.Push()` — it fires for **any project type**,
not just Go. Both `push` and `gopush` trigger it automatically:

```bash
# Any project (plain git push):
push 'implement feature X'
# → git push → ✅ Pushed: v1.2.3
# → PLAN.md detected → ✅ Jules session queued

# Go project (full workflow):
gopush 'implement feature X'
# → tests pass → git push → ✅ Pushed: v1.2.3
# → PLAN.md detected → ✅ Jules session queued
```

If `docs/PLAN.md` is absent, both commands behave as before (push only).
If Jules dispatch fails, the error appears as a warning in the summary (`⚠️ CodeJob: ...`)
but the command exits 0 — the push was successful.

See: [GOPUSH_CODEJOB_FLOW.md](codejob/diagrams/GOPUSH_CODEJOB_FLOW.md)

## State Check & Cleanup

To avoid redundant dispatches and track task progress, `CodeJob` persists an active session to the `.env` file:

```
CODEJOB=jules:SESSION_ID
CODEJOB_PR=https://github.com/owner/repo/pull/1
```

The `codejob` command becomes dual-mode:
- **If session active**: Queries Jules API. If a Pull Request is ready, it performs cleanup:
    1. `git fetch --all` to get the Jules branch.
    2. Renames `docs/PLAN.md` to `docs/CHECK_PLAN.md`.
    3. Removes `CODEJOB` from `.env`.
- **If no session**: Dispatches the task from `docs/PLAN.md`.

See: [CODEJOB_STATE_FLOW.md](codejob/diagrams/CODEJOB_STATE_FLOW.md)

## Drivers

| Driver | File | Doc |
|---|---|---|
| Jules | `code_jules.go` | [codejob/JULES_AUTOMATION.md](codejob/JULES_AUTOMATION.md) |
