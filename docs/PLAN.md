# Development Rules
- **Single Responsibility Principle (SRP):** Every file must have a single, well-defined purpose.

# Goal Description
Enhance the `goinstall` tool (part of `devflow`) so that before it installs a command via `go install`, it automatically kills any previously running daemon instance of that command. This prevents locked-file errors on Windows and ensures that the newly installed binary replaces cleanly. Uses `gorun.StopApp` from `github.com/tinywasm/gorun` for cross-platform (Linux, Windows, macOS) process termination.

> [!IMPORTANT]
> This plan depends on `gorun.StopApp` being available. Apply the [gorun PLAN.md](file:///home/cesar/Dev/Project/tinywasm/gorun/docs/PLAN.md) first and publish the new gorun version before proceeding.

# Proposed Changes

## Component: devflow/go_handler

### [MODIFY] [go.mod](file:///home/cesar/Dev/Project/tinywasm/devflow/go.mod)
Add `github.com/tinywasm/gorun` as a new dependency:
```bash
go get github.com/tinywasm/gorun@latest
```

### [MODIFY] [go_handler.go](file:///home/cesar/Dev/Project/tinywasm/devflow/go_handler.go)
In the `Install(version string)` method (line ~339), add a call to `gorun.StopApp(cmd)` **before** running `go install` for each command:

```go
import "github.com/tinywasm/gorun"

// Inside the for loop at line 339, BEFORE the go install call:
_ = gorun.StopApp(cmd) // best-effort: ignore errors (no process running is fine)
```

This silently terminates any running daemon matching the exact binary name being installed (e.g., `codejob`, `devbackup`, `goinstall`, etc.).

## Verification Plan

### Manual Verification
1. Start an instance of one of the devflow tools (e.g. `codejob`) in a separate terminal.
2. In another terminal, run `goinstall`.
3. Verify that the running `codejob` instance is forcefully terminated before its binary is successfully recompiled and installed.
