package gitgo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoHandler handles Go related operations
type GoHandler struct{}

// NewGoHandler creates a new GoHandler
func NewGoHandler() *GoHandler {
	return &GoHandler{}
}

// Tidy runs go mod tidy
func (h *GoHandler) Tidy() error {
	_, err := runCommand("go", "mod", "tidy")
	return err
}

// Verify runs go mod verify
func (h *GoHandler) Verify() error {
    _, err := runCommand("go", "mod", "verify")
    return err
}

// Test runs go test ./...
func (h *GoHandler) Test() error {
    _, err := runCommand("go", "test", "./...")
    return err
}

// TestRace runs go test -race ./...
func (h *GoHandler) TestRace() error {
    _, err := runCommand("go", "test", "-race", "./...")
    return err
}

// GetModulePath gets the current module path from go.mod
func (h *GoHandler) GetModulePath() (string, error) {
    out, err := runCommandSilent("go", "list", "-m")
    if err != nil {
        return "", fmt.Errorf("failed to get module path: %w", err)
    }
    return strings.TrimSpace(out), nil
}

// UpdateDependents updates dependent modules
func (h *GoHandler) UpdateDependents(modulePath, version, searchPath string) error {
	if searchPath == "" {
		searchPath = ".."
	}

	log("Searching dependent modules of", modulePath)
	dependents, err := h.findDependentModules(modulePath, searchPath)
	if err != nil {
		return err
	}

	if len(dependents) == 0 {
		log("No dependent modules found")
		return nil
	}

	log(fmt.Sprintf("Found %d dependent modules", len(dependents)))

	updated := 0
	for _, depDir := range dependents {
		if err := h.updateModule(depDir, modulePath, version); err != nil {
			log("Error updating", depDir, ":", err)
			continue
		}
		updated++
		log("Updated:", depDir)
	}

	log(fmt.Sprintf("Updated %d modules", updated))
	return nil
}

func (h *GoHandler) findDependentModules(modulePath, searchPath string) ([]string, error) {
	var dependents []string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() != "go.mod" {
			return nil
		}
		if h.hasDependency(path, modulePath) {
			content, _ := os.ReadFile(path)
			if !strings.Contains(string(content), "module "+modulePath) {
				dependents = append(dependents, filepath.Dir(path))
			}
		}
		return nil
	})
	return dependents, err
}

func (h *GoHandler) hasDependency(gomodPath, modulePath string) bool {
	content, err := os.ReadFile(gomodPath)
	if err != nil {
		return false
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			continue
		}
		fields := strings.Fields(line)
		for _, field := range fields {
			if field == modulePath {
				return true
			}
		}
	}
	return false
}

func (h *GoHandler) updateModule(moduleDir, dependency, version string) error {
    // Avoid os.Chdir by using Dir option in runCommand
	target := fmt.Sprintf("%s@%s", dependency, version)

    opts := &RunOptions{Dir: moduleDir}

    // go get -u
    _, err := runCommandWithOpts(opts, "go", "get", "-u", target)
    if err != nil {
         return err
    }

    // go mod tidy
    _, err = runCommandWithOpts(opts, "go", "mod", "tidy")
    return err
}
