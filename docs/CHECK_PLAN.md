# PLAN — Replace hardcoded strings with typed constants in `codejob`

## Module

`github.com/tinywasm/devflow` — located at `tinywasm/devflow/`.

## Problem

The agent that implemented the `codejob` help improvement left hardcoded string
literals scattered across library and cmd files. This violates the typed-language
contract: every repeated string must be a named constant so the compiler catches
renames, typos, and mismatches.

### Violations found

| Literal | Files | Fix |
|---|---|---|
| `"CODEJOB"` | `codejob.go` (×3), `codejob_state.go` (×4) | `const EnvKeyCodejob = "CODEJOB"` |
| `"CODEJOB_PR"` | `codejob.go` (×2), `codejob_state.go` (×5) | `const EnvKeyCodejobPR = "CODEJOB_PR"` |
| `"jules: "` | `code_jules.go:232`, `cmd/codejob/main.go:46` | `const JulesResultPrefix = "jules: "` |
| `"docs/PLAN.md"` | `cmd/codejob/main.go:65` (in help text) | use `devflow.DefaultIssuePromptPath` |
| `isEnvironmentValid()` | `cmd/codejob/main.go:74-95` | move to library as `devflow.IsEnvironmentValid(dotenvPath string) bool` |

## Rules (repeat these for the agent — zero context assumed)

- **No hardcoded strings in logic.** Every env key, file path, prefix, or flag
  name is a named constant. String literals in `switch`/`if`/`fmt.Sprintf` are
  forbidden when the value is used in more than one place.
- **Thin `cmd/`.** `cmd/*/main.go` contains only: argument parsing, DI wiring,
  and `fmt.Print`/`os.Exit`. All conditionals and validations are exported
  library functions.
- **No logic duplication.** If the library already owns a value, `cmd/` imports
  the constant — never re-derives it inline.

## Changes required

### 1. `codejob.go` — add env key constants

Add below the existing `DefaultIssuePromptPath` constant:

```go
const (
    DefaultIssuePromptPath = "docs/PLAN.md" // already exists — do not duplicate

    // EnvKeyCodejob holds the active agent session ("driver:sessionID").
    EnvKeyCodejob = "CODEJOB"
    // EnvKeyCodejobPR holds the GitHub PR URL pending merge.
    EnvKeyCodejobPR = "CODEJOB_PR"
)
```

Replace every `"CODEJOB"` and `"CODEJOB_PR"` string literal in `codejob.go`
with `EnvKeyCodejob` and `EnvKeyCodejobPR`.

### 2. `codejob_state.go` — use constants

Replace every `"CODEJOB"` and `"CODEJOB_PR"` literal with `EnvKeyCodejob` and
`EnvKeyCodejobPR`. No other logic changes.

### 3. `code_jules.go` — export result prefix

Add an exported constant and use it in the return statement:

```go
// JulesResultPrefix is the prefix on the string returned by the Jules driver
// after dispatching a session. cmd/codejob uses it to format the output.
const JulesResultPrefix = "jules: "
```

Line 232 becomes:
```go
return JulesResultPrefix + d.sessionID, nil
```

### 4. `codejob.go` — export `IsEnvironmentValid`

Add this exported function to `codejob.go` (remove it from `cmd/`):

```go
// IsEnvironmentValid reports whether the current working directory has an
// active codejob context: a running session, a pending PR, or a PLAN.md to dispatch.
// dotenvPath is the path to the .env file (typically ".env").
func IsEnvironmentValid(dotenvPath string) bool {
    if os.Getenv(EnvKeyCodejob) != "" || os.Getenv(EnvKeyCodejobPR) != "" {
        return true
    }
    env := NewDotEnv(dotenvPath)
    if val, ok := env.Get(EnvKeyCodejob); ok && val != "" {
        return true
    }
    if val, ok := env.Get(EnvKeyCodejobPR); ok && val != "" {
        return true
    }
    if _, err := os.Stat(DefaultIssuePromptPath); err == nil {
        return true
    }
    return false
}
```

### 5. `cmd/codejob/main.go` — use library symbols

**Remove** the `isEnvironmentValid()` function entirely.

Replace its call site with `devflow.IsEnvironmentValid(".env")`:
```go
if msg == "" && !devflow.IsEnvironmentValid(".env") {
    showHelp()
    return
}
```

Replace the `"jules: "` prefix check with `devflow.JulesResultPrefix`:
```go
if strings.HasPrefix(result, devflow.JulesResultPrefix) {
    sessionID := strings.TrimPrefix(result, devflow.JulesResultPrefix)
    fmt.Printf("Agent Jules • Session: %s\n", sessionID)
} else {
    fmt.Println(result)
}
```

Fix hardcoded path in `showHelp()` help text — replace the string literal
`"docs/PLAN.md"` with the constant:
```go
fmt.Printf("  1. DISPATCH: Create %s and run 'codejob' to start a new task.\n", devflow.DefaultIssuePromptPath)
```

After these changes, `cmd/codejob/main.go` must import only `devflow` and
standard library; it must contain no business logic.

## Verification

```bash
gotest ./...
```

All tests must pass. Also verify with grep that no bare `"CODEJOB"` or
`"CODEJOB_PR"` literals remain in Go source files (excluding test fixture
strings and comments):

```bash
grep -rn '"CODEJOB' --include='*.go' .
```

Expected: zero matches outside of the constant declarations themselves.

## Stages

| # | Task | Done |
|---|---|---|
| 1 | Add `EnvKeyCodejob` and `EnvKeyCodejobPR` constants to `codejob.go` | [ ] |
| 2 | Replace all `"CODEJOB"` / `"CODEJOB_PR"` literals in `codejob.go` with constants | [ ] |
| 3 | Replace all `"CODEJOB"` / `"CODEJOB_PR"` literals in `codejob_state.go` with constants | [ ] |
| 4 | Add `JulesResultPrefix` exported constant to `code_jules.go`; use it in the return | [ ] |
| 5 | Add exported `IsEnvironmentValid(dotenvPath string) bool` to `codejob.go` | [ ] |
| 6 | Remove `isEnvironmentValid()` from `cmd/codejob/main.go`; call `devflow.IsEnvironmentValid(".env")` | [ ] |
| 7 | Replace `"jules: "` literal in `cmd/codejob/main.go` with `devflow.JulesResultPrefix` | [ ] |
| 8 | Replace `"docs/PLAN.md"` literal in `showHelp()` with `devflow.DefaultIssuePromptPath` | [ ] |
| 9 | `gotest ./...` green | [ ] |
| 10 | `grep -rn '"CODEJOB' --include='*.go' .` returns zero matches outside constant declarations | [ ] |
