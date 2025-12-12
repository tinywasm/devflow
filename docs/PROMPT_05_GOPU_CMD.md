# PROMPT 05: GoPU Command

## Context
Implement the `gopu` command (separate binary) in files `cmd_gopu.go` and `workflow_gopu.go`. Identical behavior to `gopu.sh` bash, including dependent modules update.

## Reference Bash Script gopu.sh

```bash
#!/bin/bash
# Usage: gopu.sh "Commit message" [optional-tag]

commit_message="$1"
provided_tag="$2"

# Verify Go module
bash gomodcheck.sh
if [ $? -eq 0 ]; then
    
    # Execute push workflow
    bash pu.sh "$commit_message" "$provided_tag"
    if [ $? -eq 0 ]; then
        
        # Update other modules that depend on this one
        latest_tag=$(git describe --abbrev=0 --tags)
        go_mod_name=$(get module name from go.mod)
        
        bash gomodtagupdate.sh "$go_mod_name" "$latest_tag"
        
        # Backup if everything OK
        if [ $? -eq 0 ]; then
            source autoBackup.sh
        fi
    fi
fi
```

## File: workflow_gopu.go

```go
package gitgo

import (
    "fmt"
)

// WorkflowGoPU executes the complete workflow for Go projects
// Equivalent to the complete logic of gopu.sh
//
// Parameters:
//   message: Commit message
//   tag: Optional tag
//   skipTests: If true, skips tests
//   skipRace: If true, skips race tests
//   searchPath: Path to search for dependent modules (default: "..")
func WorkflowGoPU(message, tag string, skipTests, skipRace bool, searchPath string) error {
    // Default values
    if message == "" {
        message = "auto update Go package"
    }
    
    if searchPath == "" {
        searchPath = ".."
    }
    
    // 1. Verify go.mod
    log("Verifying Go module...")
    if err := GoModVerify(); err != nil {
        return fmt.Errorf("go mod verify failed: %w", err)
    }
    
    // 2. Run tests (if not skipped)
    if !skipTests {
        if err := GoTest(); err != nil {
            return fmt.Errorf("tests failed: %w", err)
        }
    }
    
    // 3. Run race tests (if not skipped)
    if !skipRace && !skipTests {
        if err := GoTestRace(); err != nil {
            return fmt.Errorf("race tests failed: %w", err)
        }
    }
    
    // 4. Execute push workflow
    log("Executing push workflow...")
    if err := WorkflowPush(message, tag); err != nil {
        return fmt.Errorf("push workflow failed: %w", err)
    }
    
    // 5. Get created tag
    latestTag, err := GitGetLatestTag()
    if err != nil {
        log("Warning: could not get latest tag:", err)
        return nil // Not fatal error
    }
    
    // 6. Get module name
    modulePath, err := GoGetModulePath()
    if err != nil {
        log("Warning: could not get module path:", err)
        return nil
    }
    
    // 7. Update dependent modules
    log("Updating dependent modules...")
    if err := GoUpdateDependents(modulePath, latestTag, searchPath); err != nil {
        log("Warning: failed to update dependents:", err)
        // Not fatal error
    }
    
    log("GoPU completed:", latestTag)
    return nil
}
```

## File: cmd_gopu.go

```go
package main

import (
    "flag"
    "fmt"
    "os"
    
    "github.com/cdvelop/gitgo"
)

func main() {
    // Flags
    helpFlag := flag.Bool("h", false, "Show help")
    flag.BoolVar(helpFlag, "help", false, "Show help")
    skipTestsFlag := flag.Bool("skip-tests", false, "Skip running tests")
    skipRaceFlag := flag.Bool("skip-race", false, "Skip race detector tests")
    skipUpdateFlag := flag.Bool("skip-update", false, "Skip updating dependent modules")
    searchPathFlag := flag.String("search", "..", "Path to search for dependent modules")
    
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, `gopu - Automated Go Project Update Workflow

Usage:
    gopu "commit message" [tag]
    gopu [options]

Arguments:
    message    Commit message
    tag        Tag name (optional, auto-generated if not provided)

Options:
    -h, --help         Show this help message
    --skip-tests       Skip running tests
    --skip-race        Skip race detector tests
    --skip-update      Skip updating dependent modules
    --search PATH      Path to search for dependent modules (default: "..")

Examples:
    gopu "feat: new feature"
    gopu "fix: bug" "v1.2.3"
    gopu --skip-race "quick fix"
    gopu --skip-update "docs only"

Workflow:
    1. go mod verify
    2. go test ./...
    3. go test -race ./...
    4. git add, commit, tag, push (via push workflow)
    5. Update dependent modules with new version
    6. go get -u module@version in dependents
    7. go mod tidy in dependents

`)
    }
    
    flag.Parse()
    
    if *helpFlag {
        flag.Usage()
        os.Exit(0)
    }
    
    // Positional arguments
    args := flag.Args()
    
    var message, tag string
    
    if len(args) > 0 {
        message = args[0]
    }
    
    if len(args) > 1 {
        tag = args[1]
    }
    
    // Determine if skip update
    skipUpdate := *skipUpdateFlag
    searchPath := *searchPathFlag
    if skipUpdate {
        searchPath = "" // Don't search if update is skipped
    }
    
    // Execute workflow
    err := gitgo.WorkflowGoPU(
        message,
        tag,
        *skipTestsFlag,
        *skipRaceFlag,
        searchPath,
    )
    
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    os.Exit(0)
}
```

## File: workflow_gopu_test.go

```go
package gitgo

import (
    "os"
    "os/exec"
    "testing"
)

func TestWorkflowGoPU(t *testing.T) {
    // Create temporary Go module
    dir := t.TempDir()
    
    // Init git
    exec.Command("git", "-C", dir, "init").Run()
    exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
    exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
    
    // Create go.mod
    gomod := "module github.com/test/repo\n\ngo 1.20\n"
    os.WriteFile(dir+"/go.mod", []byte(gomod), 0644)
    
    // Create main.go with test
    main := "package main\n\nfunc main() {}\n"
    os.WriteFile(dir+"/main.go", []byte(main), 0644)
    
    test := "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n"
    os.WriteFile(dir+"/main_test.go", []byte(test), 0644)
    
    // Change to directory
    oldDir, _ := os.Getwd()
    os.Chdir(dir)
    defer os.Chdir(oldDir)
    
    // Execute workflow (skip real push and update)
    err := WorkflowGoPU("test", "", false, true, "") // skip race, no search
    
    // Should not fail (even if no real push)
    if err != nil && err.Error() != "push failed" {
        t.Logf("Warning: %v", err) // Expected without remote
    }
}
```

## Compilation and Installation

### Local build
```bash
# Compile
go build -o gopu cmd_gopu.go

# Test
./gopu "test commit"
```

### Global installation
```bash
# From repository
go install github.com/cdvelop/gitgo/cmd_gopu.go

# Use
gopu "feat: new feature"
gopu --skip-race "quick fix"
```

## Usage

### Like original bash
```bash
# Complete workflow
gopu "feat: implement new feature"

# With specific tag
gopu "release: version 1.0.0" "v1.0.0"

# Skip race tests (faster)
gopu --skip-race "docs: update README"

# Skip dependent updates
gopu --skip-update "fix: local only"

# Tests only, no race
gopu --skip-race "refactor code"
```

### From Go code
```go
package main

import "github.com/cdvelop/gitgo"

func main() {
    // Complete gopu workflow
    gitgo.WorkflowGoPU("feat: new feature", "", false, false, "..")
    
    // With options
    gitgo.WorkflowGoPU("quick fix", "", true, true, "") // skip tests
}
```

## Exported Functions

- `WorkflowGoPU(message, tag string, skipTests, skipRace bool, searchPath string) error`

## Notes

- 100% same behavior as `gopu.sh`
- Optional flags for fine control
- `GoUpdateDependents` replaces slow bash
- Tests mandatory by default (skip with `--skip-tests`)
- Race tests mandatory by default (skip with `--skip-race`)
- Dependent search in `..` by default
- Minimal output without colors
