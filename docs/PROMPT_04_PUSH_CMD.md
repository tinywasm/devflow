# PROMPT 04: Push Command

## Context
Implement the `push` command (separate binary) and its workflow in files `cmd_push.go` and `workflow_push.go`. Identical behavior to `pu.sh` bash.

## Reference Bash Script pu.sh

```bash
#!/bin/bash
# Usage: ./pu.sh "Commit message" [optional-tag]

current_folder=$(basename "$(pwd)")
commit_message="$1"
provided_tag="$2"

# Git add
git add .

# Commit if there are changes
git commit -m "$commit_message"

# Generate or use tag
if [ -n "$provided_tag" ]; then
    new_tag="$provided_tag"
else
    new_tag=$(generate_next_tag)
fi

git tag $new_tag

# Push with upstream check
git push && git push origin $new_tag
```

## File: workflow_push.go

```go
package gitgo

import (
    "fmt"
)

// WorkflowPush executes the complete push workflow
// Equivalent to the complete logic of pu.sh
//
// Parameters:
//   message: Commit message (required)
//   tag: Optional tag (if empty, auto-generated)
func WorkflowPush(message, tag string) error {
    // Validate message
    if message == "" {
        message = "auto update package"
    }
    
    // 1. Git add
    if err := GitAdd(); err != nil {
        return fmt.Errorf("git add failed: %w", err)
    }
    
    // 2. Commit (only if there are changes)
    if err := GitCommit(message); err != nil {
        return fmt.Errorf("git commit failed: %w", err)
    }
    
    // 3. Determine tag (provided or generated)
    finalTag := tag
    if finalTag == "" {
        generatedTag, err := GitGenerateNextTag()
        if err != nil {
            return fmt.Errorf("failed to generate tag: %w", err)
        }
        finalTag = generatedTag
    }
    
    // 4. Create tag
    if err := GitCreateTag(finalTag); err != nil {
        // If it already exists, not fatal error (can be re-run)
        log("Warning:", err)
    }
    
    // 5. Push commits and tag
    if err := GitPushWithTags(finalTag); err != nil {
        return fmt.Errorf("push failed: %w", err)
    }
    
    log("Push completed:", finalTag)
    return nil
}
```

## File: cmd_push.go

```go
package main

import (
    "flag"
    "fmt"
    "os"
    
    "github.com/cdvelop/gitgo"
)

func main() {
    // Parse flags (keep simple like bash)
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, `push - Automated Git workflow

Usage:
    push "commit message" [tag]
    push [options]

Arguments:
    message    Commit message (required if no changes)
    tag        Tag name (optional, auto-generated if not provided)

Options:
    -h, --help     Show this help message

Examples:
    push "feat: new feature"
    push "fix: bug correction" "v1.2.3"

Workflow:
    1. git add .
    2. git commit -m "message"
    3. git tag <tag> (auto-generated or provided)
    4. git push && git push origin <tag>

`)
    }
    
    // Flag -h or --help
    helpFlag := flag.Bool("h", false, "Show help")
    flag.BoolVar(helpFlag, "help", false, "Show help")
    flag.Parse()
    
    if *helpFlag {
        flag.Usage()
        os.Exit(0)
    }
    
    // Positional arguments (like original bash)
    args := flag.Args()
    
    var message, tag string
    
    if len(args) > 0 {
        message = args[0]
    }
    
    if len(args) > 1 {
        tag = args[1]
    }
    
    // Execute workflow
    if err := gitgo.WorkflowPush(message, tag); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    os.Exit(0)
}
```

## File: workflow_push_test.go

```go
package gitgo

import (
    "os"
    "os/exec"
    "testing"
)

func TestWorkflowPush(t *testing.T) {
    // Create temporary repo
    dir := t.TempDir()
    
    // Init git
    exec.Command("git", "-C", dir, "init").Run()
    exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
    exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
    
    // Change to directory
    oldDir, _ := os.Getwd()
    os.Chdir(dir)
    defer os.Chdir(oldDir)
    
    // Create file
    os.WriteFile("test.txt", []byte("test"), 0644)
    
    // Execute workflow (without real push to avoid remote error)
    // Only test up to tag
    GitAdd()
    GitCommit("test commit")
    tag, _ := GitGenerateNextTag()
    
    if tag != "v0.0.1" {
        t.Errorf("Expected v0.0.1, got %s", tag)
    }
    
    GitCreateTag(tag)
    
    exists, _ := GitTagExists(tag)
    if !exists {
        t.Error("Tag should exist")
    }
}
```

## Compilation and Installation

### Local build
```bash
# Compile
go build -o push cmd_push.go

# Test
./push "test commit"
```

### Global installation
```bash
# From repository
go install github.com/cdvelop/gitgo/cmd_push.go

# Use
push "feat: new feature"
push "fix: bug" "v1.0.0"
```

### Build with Makefile (optional)
```makefile
.PHONY: build-push
build-push:
	go build -o bin/push cmd_push.go

.PHONY: install-push
install-push:
	go install cmd_push.go
```

## Usage

### Like original bash
```bash
# Auto-generate tag
push "feat: implement new feature"

# Specific tag
push "release: version 1.0.0" "v1.0.0"

# No message (uses default)
push
```

### From Go code
```go
package main

import "github.com/cdvelop/gitgo"

func main() {
    // Push workflow
    gitgo.WorkflowPush("feat: new feature", "")
    
    // With specific tag
    gitgo.WorkflowPush("release v2", "v2.0.0")
}
```

## Exported Functions

- `WorkflowPush(message, tag string) error`

## Notes

- 100% same behavior as `pu.sh`
- Positional arguments (no complex flags)
- Minimal output
- No colors
- Basic error validation
- Basic tests (~40%)
