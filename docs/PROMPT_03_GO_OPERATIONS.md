# PROMPT 03: Go Operations

## Context
Implement Go-specific operations in `go_operations.go` and `go_mod_update.go` files. Uses `RunCommand` from `executor.go`.

## Reference Bash Scripts

### gomodcheck.sh
```bash
go mod verify
```

### gopu.sh - Tests
```bash
go test ./...
go test -race ./...
```

### gomodtagupdate.sh - Update dependents
```bash
# Find modules that use this package
# Update go.mod in those modules
go get -u github.com/user/repo@v1.2.3
go mod tidy
```

## File: go_operations.go

```go
package gitgo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoModVerify verifies go.mod integrity
func GoModVerify() error {
	if !goModExists() {
		return fmt.Errorf("go.mod not found")
	}
	
	_, err := RunCommand("go", "mod", "verify")
	return err
}

// GoModTidy runs go mod tidy
func GoModTidy() error {
	_, err := RunCommand("go", "mod", "tidy")
	return err
}

// GoTest runs tests
func GoTest() error {
	log("Running tests...")
	// CombinedOutput is handled by RunCommand, but we might want to see output even on success?
	// RunCommand logs the command. If it fails, it returns error with output.
	// If we want to see "PASS" output, we might need to print the result.
	output, err := RunCommand("go", "test", "./...")
	if err != nil {
		return err
	}
	// Optional: log output if verbose
	return nil
}

// GoTestRace runs tests with race detector
func GoTestRace() error {
	log("Running race detector...")
	_, err := RunCommand("go", "test", "-race", "./...")
	return err
}

// GoGetModuleName gets module name from go.mod
// Extracts: module github.com/user/repo -> repo
func GoGetModuleName() (string, error) {
    modPath, err := GoGetModulePath()
    if err != nil {
        return "", err
    }
    
    // Extract last part of path
    parts := strings.Split(modPath, "/")
    if len(parts) == 0 {
        return "", fmt.Errorf("invalid module path: %s", modPath)
    }
    
    return parts[len(parts)-1], nil
}

// GoGetModulePath gets full module path
// Example: github.com/cdvelop/gitgo
func GoGetModulePath() (string, error) {
    file, err := os.Open("go.mod")
    if err != nil {
        return "", err
    }
    defer file.Close()
    
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if strings.HasPrefix(line, "module ") {
            return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
        }
    }
    
    return "", fmt.Errorf("module directive not found in go.mod")
}

// goModExists checks if go.mod exists
func goModExists() bool {
    _, err := os.Stat("go.mod")
    return err == nil
}
```

## File: go_mod_update.go

```go
package gitgo

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

// GoUpdateDependents updates modules that depend on the current one
// Rewrite in Go of gomodtagupdate.sh (much faster)
// 
// Parameters:
//   modulePath: Current module path (eg: github.com/cdvelop/gitgo)
//   version: New version/tag (eg: v0.0.5)
//   searchPath: Directory to search for dependent modules (eg: "..")
func GoUpdateDependents(modulePath, version, searchPath string) error {
    if searchPath == "" {
        searchPath = ".."
    }
    
    log("Searching dependent modules of", modulePath)
    
    // Find modules that depend on current
    dependents, err := findDependentModules(modulePath, searchPath)
    if err != nil {
        return err
    }
    
    if len(dependents) == 0 {
        log("No dependent modules found")
        return nil
    }
    
    log(fmt.Sprintf("Found %d dependent modules", len(dependents)))
    
    // Update each dependent
    updated := 0
    for _, depDir := range dependents {
        if err := updateModule(depDir, modulePath, version); err != nil {
            log("Error updating", depDir, ":", err)
            continue
        }
        updated++
        log("Updated:", depDir)
    }
    
    log(fmt.Sprintf("Updated %d modules", updated))
    return nil
}

// findDependentModules searches for modules that have modulePath as dependency
func findDependentModules(modulePath, searchPath string) ([]string, error) {
    var dependents []string
    
    err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil // Continue despite errors
        }
        
        // Only go.mod files
        if info.Name() != "go.mod" {
            return nil
        }
        
        // Don't process current directory's go.mod
        absCurrentDir, _ := filepath.Abs(".")
        absPath, _ := filepath.Abs(filepath.Dir(path))
        if absPath == absCurrentDir {
            return nil
        }
        
        // Check if this go.mod has the dependency
        if hasDependency(path, modulePath) {
            dependents = append(dependents, filepath.Dir(path))
        }
        
        return nil
    })
    
    return dependents, err
}

// hasDependency checks if a go.mod contains a specific dependency
func hasDependency(gomodPath, modulePath string) bool {
    content, err := os.ReadFile(gomodPath)
    if err != nil {
        return false
    }
    
    // Search for module path in content
    return strings.Contains(string(content), modulePath)
}

// updateModule updates a specific module to a new version
func updateModule(moduleDir, dependency, version string) error {
    // Change to module directory
    originalDir, err := os.Getwd()
    if err != nil {
        return err
    }
    defer os.Chdir(originalDir)
    
    if err := os.Chdir(moduleDir); err != nil {
        return err
    }
    
    // go get -u dependency@version
    target := fmt.Sprintf("%s@%s", dependency, version)
    cmd := exec.Command("go", "get", "-u", target)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("go get failed: %w", err)
    }
    
    // go mod tidy
    cmd = exec.Command("go", "mod", "tidy")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("go mod tidy failed: %w", err)
    }
    
    return nil
}
```

## File: go_operations_test.go

```go
package gitgo

import (
    "os"
    "os/exec"
    "testing"
)

func createTestGoModule(t *testing.T, moduleName string) string {
    t.Helper()
    
    dir := t.TempDir()
    
    // Create go.mod
    gomod := "module " + moduleName + "\n\ngo 1.20\n"
    os.WriteFile(dir+"/go.mod", []byte(gomod), 0644)
    
    // Create basic main.go
    main := "package main\n\nfunc main() {}\n"
    os.WriteFile(dir+"/main.go", []byte(main), 0644)
    
    return dir
}

func TestGoGetModulePath(t *testing.T) {
    dir := createTestGoModule(t, "github.com/test/repo")
    os.Chdir(dir)
    
    path, err := GoGetModulePath()
    if err != nil {
        t.Fatal(err)
    }
    
    if path != "github.com/test/repo" {
        t.Errorf("Expected github.com/test/repo, got %s", path)
    }
}

func TestGoGetModuleName(t *testing.T) {
    dir := createTestGoModule(t, "github.com/test/myrepo")
    os.Chdir(dir)
    
    name, err := GoGetModuleName()
    if err != nil {
        t.Fatal(err)
    }
    
    if name != "myrepo" {
        t.Errorf("Expected myrepo, got %s", name)
    }
}

func TestGoModVerify(t *testing.T) {
    dir := createTestGoModule(t, "github.com/test/repo")
    os.Chdir(dir)
    
    err := GoModVerify()
    if err != nil {
        t.Fatal(err)
    }
}

func TestGoTest(t *testing.T) {
    dir := createTestGoModule(t, "github.com/test/repo")
    
    // Create passing test
    testContent := `package main
import "testing"
func TestExample(t *testing.T) {}
`
    os.WriteFile(dir+"/main_test.go", []byte(testContent), 0644)
    
    os.Chdir(dir)
    
    err := GoTest()
    if err != nil {
        t.Fatal(err)
    }
}
```

## File: go_mod_update_test.go

```go
package gitgo

import (
    "os"
    "testing"
)

func TestFindDependentModules(t *testing.T) {
    // Create temporary directory structure
    tmpDir := t.TempDir()
    
    // Main module
    mainDir := tmpDir + "/main"
    os.MkdirAll(mainDir, 0755)
    os.WriteFile(mainDir+"/go.mod", []byte("module github.com/test/main\n\ngo 1.20\n"), 0644)
    
    // Dependent module 1
    dep1Dir := tmpDir + "/dep1"
    os.MkdirAll(dep1Dir, 0755)
    dep1Mod := `module github.com/test/dep1

go 1.20

require github.com/test/main v0.0.1
`
    os.WriteFile(dep1Dir+"/go.mod", []byte(dep1Mod), 0644)
    
    // Independent module
    indepDir := tmpDir + "/indep"
    os.MkdirAll(indepDir, 0755)
    os.WriteFile(indepDir+"/go.mod", []byte("module github.com/test/indep\n\ngo 1.20\n"), 0644)
    
    // Search dependents
    dependents, err := findDependentModules("github.com/test/main", tmpDir)
    if err != nil {
        t.Fatal(err)
    }
    
    // Should find only dep1
    if len(dependents) != 1 {
        t.Errorf("Expected 1 dependent, got %d", len(dependents))
    }
}

func TestHasDependency(t *testing.T) {
    tmpDir := t.TempDir()
    gomodPath := tmpDir + "/go.mod"
    
    content := `module github.com/test/repo

go 1.20

require github.com/cdvelop/gitgo v0.0.1
`
    os.WriteFile(gomodPath, []byte(content), 0644)
    
    // Should find the dependency
    if !hasDependency(gomodPath, "github.com/cdvelop/gitgo") {
        t.Error("Expected to find dependency")
    }
    
    // Should not find this one
    if hasDependency(gomodPath, "github.com/other/repo") {
        t.Error("Should not find non-existent dependency")
    }
}
```

## Exported Functions

### go_operations.go
- `GoModVerify()`
- `GoModTidy()`
- `GoTest()`
- `GoTestRace()`
- `GoGetModuleName()`
- `GoGetModulePath()`

### go_mod_update.go
- `GoUpdateDependents(modulePath, version, searchPath)`

## Notes

- `GoUpdateDependents` is critical - replaces slow bash
- Recursively searches in `searchPath` for all go.mod files
- Automatically updates with `go get -u` and `go mod tidy`
- Minimal output with `log()`
- Basic tests for core functionality
