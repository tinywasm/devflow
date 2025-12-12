# PROMPT 01: General Architecture of GitGo Project

## Context
Create a Go library called `gitgo` that reimplements the functionality of bash scripts `pu.sh` and `gopu.sh` in a simple, testable and reusable way for TUI tool integration.

## Objectives
1. **Two separate installable binaries** via `go install`:
   - `push`: Equivalent to `pu.sh`
   - `gopu`: Equivalent to `gopu.sh`

2. **Reusable library**: Other projects should be able to import and use `gitgo` as a dependency

3. **Simple logger**: `func(...any)` function configurable via `SetLogger`

4. **~40% Testable**: Basic functionality with essential tests

## Directory Structure (No Subfolders)

```
gitgo/
├── go.mod
├── LICENSE
├── README.md
├── docs/
│   ├── PROMPT_01_ARCHITECTURE.md
│   ├── PROMPT_02_GIT_OPERATIONS.md
│   ├── PROMPT_03_GO_OPERATIONS.md
│   ├── PROMPT_04_PUSH_CMD.md
│   ├── PROMPT_05_GOPU_CMD.md
│   └── PROMPT_06_TESTING.md
├── git_operations.go       # Git operations (add, commit, tag, push)
├── git_operations_test.go
├── go_operations.go        # Go operations (test, mod, verify)
├── go_operations_test.go
├── go_mod_update.go        # Dependents update (gomodtagupdate)
├── go_mod_update_test.go
├── workflow_push.go        # Push command workflow
├── workflow_push_test.go
├── workflow_gopu.go        # GoPU command workflow
├── workflow_gopu_test.go
├── cmd_push.go             # Push binary (package main)
├── cmd_gopu.go             # GoPU binary (package main)
├── logger.go               # Simple logger (func(...any))
├── executor.go             # Testable exec.Command wrapper
├── executor_test.go
└── helpers.go              # General utilities
```

## Main Interfaces

### 1. Logger Interface
```go
type Logger interface {
    Info(msg string, args ...any)
    Success(msg string, args ...any)
    Warning(msg string, args ...any)
    Error(msg string, args ...any)
}
```

### 2. GitHandler Interface
```go
type GitHandler interface {
    // Tag operations
    GetLatestTag() (string, error)
    CreateTag(tag, message string) error
    PushTag(tag string) error
    TagExists(tag string) (bool, error)
    GenerateNextTag() (string, error)
    
    // Commit operations
   Main Components

### 1. Simple Logger
```go
// LogFunc simple logging function
type LogFunc func(v ...any)

// defaultLog default implementation with fmt
var defaultLog LogFunc = func(v ...any) {
    fmt.Println(v...)
}

// SetLogger configures custom log function
func SetLogger(fn LogFunc) {
    defaultLog = fn
}

// log internal function that uses defaultLog
func log(v ...any) {
    defaultLog(v...)
}
```

### 2. Git Operations
```go
// Main Git operations
func GitAdd() error
func GitCommit(message string) error
func GitHasChanges() (bool, error)
func GitGetLatestTag() (string, error)
func GitCreateTag(tag string) error
func GitGenerateNextTag() (string, error)
func GitPush() error
func GitPushTag(tag string) error
func GitGetCurrentBranch() (string, error)
func GitHasUpstream() (bool, error)
func GitSetUpstream(branch string) error
```

### 3. Go Operations
```gobinaries
go install github.com/cdvelop/gitgo/cmd_push.go@latest
go install github.com/cdvelop/gitgo/cmd_gopu.go@latest
```

### CLI Usage (same as bash)
```bash
# Push command (like pu.sh)
push "feat: implement new feature"
push "fix: bug correction" "v1.2.3"

# GoPU command (like gopu.sh)
gopu "refactor: improve performance"
gopu "docs: update README" "v2.0.0"
```

### Library Usage in TUI
```go
package main

import (
    "github.com/cdvelop/gitgo"
)

func main() {
    // Configure custom logger for TUI
    gitgo.SetLogger(func(v ...any) {
        // Your TUI logic here
        myTUI.AppendLog(fmt.Sprint(v...))
    })
    
    // Execute push workflow
    if err := gitgo.WorkflowPush("feat: new feature", ""); err != nil {
        log.Fatal(
- **internal/**: Non-exportable internal code

### 2. Dependency Injection
- Logger configurable via `SetLogger()`
- Allows integration with custom loggers (logrus, zap, etc)
- Default implementation uses `fmt` package

### 3. Testability
- Interfaces for all main operations
- Mocks generatable with `mockgen`
- Wrapper for `exec.Command` in `internal/executor`

### 4. Versioning
- Follows in Subfolders
- Everything in project root
- Descriptive filenames based on single responsibility
- Simpler to navigate and understand

### 2. Simple Logger
- Only `func(...any)` function
- Configurable via `SetLogger()`
- Default: `fmt.Println`
- No colors, minimal output

### 3. Basic Testability
- ~40% coverage
- Tests for critical functionality
- Mock of `exec.Command` for tests

### 4. No External Dependencies
- Only Go stdlib
- No CLI frameworks
- No logging libraries
- No testing libraries (only `testing`)

### 5. Bash-Compatible Flags
- Same behavior as `pu.sh` and `gopu.sh`
- Positional arguments: `push "message" [tag]`
- No complex flags

### 6. Module Updates
- `GoUpdateDependents` rewritten in Go (faster than bash)
- Automatically searches and updates dependent modules

## Dependencies (Only Stdlib)
coverage ~40% (basic functionality)
2. ✅ Basic GoDoc documentation
3. ✅ README with examples
4. ✅ Binaries installable via `go install`
5. ✅ Simple configurable logger
6. ✅ No external dependencies (only stdlib)
7. ✅ Identical behavior to bash scripts
8. ✅ Minimal output without colors
9. ✅ Compatible with Go 1.20+

## Next Steps

1. Implement git operations (PROMPT_02)
2. Implement go operations (PROMPT_03)
3. Implement push command (PROMPT_04)
4. Implement gopu command (PROMPT_05)
5. Basic tests (PROMPT_06)

## Files by Responsibility

- `git_operations.go`: Only Git operations
- `go_operations.go`: Only Go operations
- `go_mod_update.go`: Only dependents update
- `workflow_push.go`: Only push workflow logic
- `workflow_gopu.go`: Only gopu workflow logic
- `cmd_push.go`: Only push CLI
- `cmd_gopu.go`: Only gopu CLI
- `logger.go`: Only simple logger
- `executor.go`: Only exec wrapper
- `helpers.go`: General utilities

## Notes

- Maintain 100% compatibility with bash scripts
- Prioritize speed over features
- Minimal output for TUI integration
- No colors in output