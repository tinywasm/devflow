# PROMPT 06: Basic Testing (~40%)

## Context
Simple testing strategy focused on critical functionality. No external frameworks, only `testing` stdlib. Target coverage ~40%.

## Test Structure

```
gitgo/
├── git_operations_test.go
├── go_operations_test.go
├── go_mod_update_test.go
├── workflow_push_test.go
├── workflow_gopu_test.go
└── logger_test.go
```

## Priority Tests

### 1. Git Operations (Critical)
- ✅ `GitHasChanges`
- ✅ `GitGenerateNextTag`
- ✅ `GitCommit`
- ⚠️ `GitPush` (mock or skip)

### 2. Go Operations (Critical)
- ✅ `GoGetModulePath`
- ✅ `GoGetModuleName`
- ✅ `GoModVerify`
- ⚠️ `GoTest` (basic)

### 3. Go Mod Update (Very Critical)
- ✅ `findDependentModules`
- ✅ `hasDependency`
- ⚠️ `updateModule` (can be slow)

### 4. Workflows (Basic)
- ✅ Push workflow up to tag
- ✅ Basic gopu workflow
- ⚠️ Real push omitted (no remote)

## Test Helpers

### File: helpers.go

```go
package gitgo

import (
    "os"
    "os/exec"
    "path/filepath"
)

// testCreateGitRepo creates a temporary Git repo for tests
// For internal use in tests only
func testCreateGitRepo() (dir string, cleanup func()) {
    dir, _ = os.MkdirTemp("", "gitgo-test-")
    
    // Init git
    exec.Command("git", "-C", dir, "init").Run()
    exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
    exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
    
    cleanup = func() {
        os.RemoveAll(dir)
    }
    
    return dir, cleanup
}

// testCreateGoModule creates a temporary Go module
func testCreateGoModule(moduleName string) (dir string, cleanup func()) {
    dir, _ = os.MkdirTemp("", "gitgo-gomod-")
    
    // Create go.mod
    gomod := "module " + moduleName + "\n\ngo 1.20\n"
    os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)
    
    // Create main.go
    main := "package main\n\nfunc main() {}\n"
    os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0644)
    
    cleanup = func() {
        os.RemoveAll(dir)
    }
    
    return dir, cleanup
}
```

## Complete Test Examples

### git_operations_test.go (excerpt)

```go
package gitgo

import (
    "os"
    "os/exec"
    "testing"
)

func TestGitGenerateNextTag_NoTags(t *testing.T) {
    dir, cleanup := testCreateGitRepo()
    defer cleanup()
    
    oldDir, _ := os.Getwd()
    os.Chdir(dir)
    defer os.Chdir(oldDir)
    
    // Initial commit required
    os.WriteFile("test.txt", []byte("test"), 0644)
    exec.Command("git", "add", ".").Run()
    exec.Command("git", "commit", "-m", "init").Run()
    
    // Without tags should generate v0.0.1
    tag, err := GitGenerateNextTag()
    
    if err != nil {
        t.Fatal(err)
    }
    
    if tag != "v0.0.1" {
        t.Errorf("Expected v0.0.1, got %s", tag)
    }
}

func TestGitGenerateNextTag_WithExisting(t *testing.T) {
    dir, cleanup := testCreateGitRepo()
    defer cleanup()
    
    oldDir, _ := os.Getwd()
    os.Chdir(dir)
    defer os.Chdir(oldDir)
    
    // Setup
    os.WriteFile("test.txt", []byte("test"), 0644)
    exec.Command("git", "add", ".").Run()
    exec.Command("git", "commit", "-m", "init").Run()
    exec.Command("git", "tag", "v0.0.5").Run()
    
    // Should generate v0.0.6
    tag, err := GitGenerateNextTag()
    
    if err != nil {
        t.Fatal(err)
    }
    
    if tag != "v0.0.6" {
        t.Errorf("Expected v0.0.6, got %s", tag)
    }
}

func TestGitHasChanges(t *testing.T) {
    dir, cleanup := testCreateGitRepo()
    defer cleanup()
    
    oldDir, _ := os.Getwd()
    os.Chdir(dir)
    defer os.Chdir(oldDir)
    
    // Initial commit
    os.WriteFile("test.txt", []byte("test"), 0644)
    exec.Command("git", "add", ".").Run()
    exec.Command("git", "commit", "-m", "init").Run()
    
    // No changes
    has, err := GitHasChanges()
    if err != nil {
        t.Fatal(err)
    }
    if has {
        t.Error("Should not have changes")
    }
    
    // With changes
    os.WriteFile("test.txt", []byte("modified"), 0644)
    GitAdd()
    
    has, err = GitHasChanges()
    if err != nil {
        t.Fatal(err)
    }
    if !has {
        t.Error("Should have changes")
    }
}
```

### go_mod_update_test.go (excerpt)

```go
package gitgo

import (
    "os"
    "path/filepath"
    "testing"
)

func TestHasDependency(t *testing.T) {
    dir, cleanup := testCreateGoModule("github.com/test/main")
    defer cleanup()
    
    gomodPath := filepath.Join(dir, "go.mod")
    
    // Add dependency
    content := `module github.com/test/main

go 1.20

require github.com/cdvelop/gitgo v0.0.1
`
    os.WriteFile(gomodPath, []byte(content), 0644)
    
    // Should find the dependency
    if !hasDependency(gomodPath, "github.com/cdvelop/gitgo") {
        t.Error("Expected to find dependency")
    }
    
    // Should not find this one
    if hasDependency(gomodPath, "github.com/other/pkg") {
        t.Error("Should not find non-existent dependency")
    }
}

func TestFindDependentModules(t *testing.T) {
    // Create temporary structure
    tmpDir, _ := os.MkdirTemp("", "gitgo-deps-")
    defer os.RemoveAll(tmpDir)
    
    // Main module
    mainDir := filepath.Join(tmpDir, "main")
    os.MkdirAll(mainDir, 0755)
    os.WriteFile(filepath.Join(mainDir, "go.mod"), 
        []byte("module github.com/test/main\n\ngo 1.20\n"), 0644)
    
    // Dependent 1
    dep1Dir := filepath.Join(tmpDir, "dep1")
    os.MkdirAll(dep1Dir, 0755)
    dep1Mod := `module github.com/test/dep1

go 1.20

require github.com/test/main v0.0.1
`
    os.WriteFile(filepath.Join(dep1Dir, "go.mod"), []byte(dep1Mod), 0644)
    
    // Independent
    indepDir := filepath.Join(tmpDir, "indep")
    os.MkdirAll(indepDir, 0755)
    os.WriteFile(filepath.Join(indepDir, "go.mod"),
        []byte("module github.com/test/indep\n\ngo 1.20\n"), 0644)
    
    // Search
    oldDir, _ := os.Getwd()
    os.Chdir(mainDir)
    defer os.Chdir(oldDir)
    
    dependents, err := findDependentModules("github.com/test/main", tmpDir)
    if err != nil {
        t.Fatal(err)
    }
    
    // Should find only dep1
    if len(dependents) != 1 {
        t.Errorf("Expected 1 dependent, got %d", len(dependents))
    }
}
```

### logger_test.go

```go
package gitgo

import (
    "testing"
)

func TestSetLogger(t *testing.T) {
    // Capture logs
    var logged []any
    
    customLog := func(v ...any) {
        logged = append(logged, v...)
    }
    
    SetLogger(customLog)
    
    log("test", "message")
    
    if len(logged) != 2 {
        t.Errorf("Expected 2 logged items, got %d", len(logged))
    }
    
    if logged[0] != "test" {
        t.Errorf("Expected 'test', got %v", logged[0])
    }
}
```

## Testing Commands

### Makefile

```makefile
.PHONY: test
test:
	go test -v ./...

.PHONY: test-cover
test-cover:
	go test -cover ./...

.PHONY: test-verbose
test-verbose:
	go test -v -cover ./...
```

### Execution

```bash
# Basic tests
go test -v

# With coverage
go test -cover

# Specific tests
go test -v -run TestGitGenerateNextTag

# Fast tests only
go test -short -v
```

## Expected Coverage

| File | Target Coverage |
|------|----------------|
| git_operations.go | ~50% |
| go_operations.go | ~40% |
| go_mod_update.go | ~40% |
| workflow_push.go | ~30% |
| workflow_gopu.go | ~25% |
| logger.go | ~60% |
| **TOTAL** | **~40%** |

## Omitted Tests (Optional/Future)

- Real push to remote (requires configuration)
- Complete integration tests
- Benchmarks
- Complete race detector tests

## Notes

- Fast tests (< 5 seconds total)
- No external dependencies
- Only `testing` stdlib
- ~40% coverage sufficient
- Priority on critical functionality
- Tests may fail if git is not configured
