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

### Jules Auth
Jules API key is managed via the system keyring (`github.com/zalando/go-keyring`):
- **After first run**: reads silently from keyring — no env vars required in local use

### GitHub Auth
To prevent interactive Device Flow prompts during `codejob` (which can block the flow), `devflow` uses a fine-grained Personal Access Token (PAT) stored in the keyring to recover the `gh` session automatically if it expires.

**Setup:**
1. Create a **fine-grained PAT** at [github.com/settings/tokens](https://github.com/settings/tokens) with the following permissions:
   - **Contents**: Read/Write
   - **Pull requests**: Read/Write
2. The first time `codejob` detects an expired session, it will prompt you for this PAT and save it in the keyring.
3. To rotate the token, run: `codejob --reset-gh-token`.

### Auto-Detection
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

## `codejob` vs `gopush` — not alternatives

**`gopush` publishes. `codejob` runs the plan loop** (dispatch → review → close) and calls
`gopush` for you at the end.

| You did… | Publish with |
|---|---|
| Edited docs, or a small fix — **no plan** | **`gopush 'message'`** — there is no PR for codejob to close |
| Wrote `docs/PLAN.md` and dispatched it | **`codejob 'message'`** — merges the PR, calls `gopush`, deletes `CHECK_PLAN.md` |

⚠️ Bare `codejob` (no arguments) **dispatches** `docs/PLAN.md` to the execution agent. It is
not a dry-run, a lint, or a way to inspect an error.

## `docs/PLAN.md` frontmatter (REQUIRED — dispatch fails without it)

Every `docs/PLAN.md` must **open** with a frontmatter block. The very first line of the file
is `---`, before any heading or blockquote:

```markdown
---
message: "feat: what this plan implements"
tag: v0.2.0
---

# Plan — ...
```

| Key | Required | Meaning |
|-----|----------|---------|
| `message` | **yes** | Commit message used when the loop is closed (`codejob 'msg'` overrides it). |
| `tag` | no | Explicit version (`v0.2.0`). Omitted → `gopush` auto-bumps. |

Unknown keys are ignored. Values may be quoted or bare.

Without it, dispatch aborts with:

```
Error: invalid plan frontmatter in docs/PLAN.md: plan frontmatter: file must start with a '---' line
```

## Usage

### CLI
```bash
go install github.com/tinywasm/devflow/cmd/codejob@latest

# ── DESPACHAR ──────────────────────────────────────────────────────────────
# Envía docs/PLAN.md al agente externo (Jules).
# El wizard de setup corre automáticamente si falta el API key.
codejob

# ── PUBLICAR (cerrar el loop) ───────────────────────────────────────────────
# Ejecutar DESPUÉS de revisar y aprobar el PR abierto por Jules.
# Fusiona el PR y publica via gopush (git tag + push).
# Si existe un nuevo docs/PLAN.md, despacha el siguiente job automáticamente.
codejob 'feat: implemented feature'
codejob 'feat: implemented feature' v0.3.0  # con tag explícito
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

`codejob` uses `gopush` to sync changes before dispatch and to publish changes when "closing the loop". It inherits all `gopush` behavior, including **automatic internal submodule syncing** when publishing a release.

## State Check & Cleanup

To avoid redundant dispatches and track task progress, `CodeJob` persists an active session to the `.env` file:

```
CODEJOB=jules:SESSION_ID
CODEJOB_PR=https://github.com/owner/repo/pull/1
```

The `codejob` command becomes dual-mode:
- **If session active**: Queries Jules API. If a Pull Request is ready, it performs cleanup:
    1. `CheckoutPRBranch`: fetches, stashes local drift, and hard-positions the tree on the Jules branch. This is **transactional**: if checkout or stashing fails, cleanup aborts and state is preserved for retry.
    2. Renames `docs/PLAN.md` to `docs/CHECK_PLAN.md`.
    3. Removes `CODEJOB` from `.env`.
    4. Sets `CODEJOB_PR` in `.env`.
- **If no session**: Dispatches the task from `docs/PLAN.md`.

## Drivers

| Driver | File | Doc |
|---|---|---|
| Jules | `code_jules.go` | [codejob/JULES_AUTOMATION.md](codejob/JULES_AUTOMATION.md) |
