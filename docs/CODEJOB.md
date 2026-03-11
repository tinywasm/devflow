# CodeJob

`CodeJob` is a chain-of-responsibility orchestrator that sends a coding task (defined in an `PLAN.md` file) to a sequence of external AI agent drivers.

## Architecture

See: [CODEJOB_FLOW.md](diagrams/CODEJOB_FLOW.md)

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

Dispatching a task automatically runs the setup wizard if the Jules API key is missing from your system keyring.

## Authentication & Auto-Detection

Jules API key is managed via the system keyring (`github.com/zalando/go-keyring`):
- **After first run**: reads silently from keyring — no env vars required in local use

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

# Dispatch (default path: docs/PLAN.md)
# Auto-setup wizard runs if API key is missing.
codejob

# Close the loop after reviewing PR
# Merges PR and publishes via gopush.
# If a new docs/PLAN.md exists, this automatically dispatches the next job.
codejob 'fix: implemented feature'
codejob 'fix: implemented feature' v0.3.0  # with explicit tag
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

## Integrated Flow (via gopush)

`codejob` uses `gopush` to sync changes before dispatch and to publish changes when "closing the loop".

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
    4. Sets `CODEJOB_PR` in `.env`.
- **If no session**: Dispatches the task from `docs/PLAN.md`.

## Drivers

| Driver | File | Doc |
|---|---|---|
| Jules | `code_jules.go` | [codejob/JULES_AUTOMATION.md](codejob/JULES_AUTOMATION.md) |
