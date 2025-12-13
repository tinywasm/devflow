package gitgo

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Go handler for Go operations
type Go struct {
	git *Git
}

// NewGo creates a new Go handler
func NewGo(gitHandler *Git) *Go {
	return &Go{
		git: gitHandler,
	}
}

// Push executes the complete workflow for Go projects
// Parameters:
//   message: Commit message
//   tag: Optional tag
//   skipTests: If true, skips tests
//   skipRace: If true, skips race tests
//   searchPath: Path to search for dependent modules (default: "..")
func (g *Go) Push(message, tag string, skipTests, skipRace bool, searchPath string) (string, error) {
	// Default values
	if message == "" {
		message = "auto update Go package"
	}

	if searchPath == "" {
		searchPath = ".."
	}

	summary := []string{}

	// 1. Verify go.mod
	if err := g.verify(); err != nil {
		return "", fmt.Errorf("go mod verify failed: %w", err)
	}
	summary = append(summary, "Verified go.mod")

	// 2. Run tests (if not skipped)
	if !skipTests {
		if err := g.test(); err != nil {
			return "", fmt.Errorf("tests failed: %w", err)
		}
		summary = append(summary, "Tests passed")
	} else {
		summary = append(summary, "Tests skipped")
	}

	// 3. Run race tests (if not skipped)
	if !skipRace && !skipTests {
		if err := g.testRace(); err != nil {
			return "", fmt.Errorf("race tests failed: %w", err)
		}
		summary = append(summary, "Race tests passed")
	} else if skipRace && !skipTests {
		summary = append(summary, "Race tests skipped")
	}

	// 4. Execute push workflow
	pushSummary, err := g.git.Push(message, tag)
	if err != nil {
		return "", fmt.Errorf("push workflow failed: %w", err)
	}
	summary = append(summary, fmt.Sprintf("Git Push [%s]", pushSummary))

	// 5. Get created tag (from Git handler or just get latest)
	latestTag, err := g.git.GetLatestTag()
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: could not get latest tag: %v", err))
		// Not fatal error
	}

	// 6. Get module name
	modulePath, err := g.getModulePath()
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: could not get module path: %v", err))
		return strings.Join(summary, ", "), nil
	}

	// 7. Update dependent modules
	updated, err := g.updateDependents(modulePath, latestTag, searchPath)
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: failed to update dependents: %v", err))
		// Not fatal error
	}
	summary = append(summary, fmt.Sprintf("Updated %d dependent modules", updated))

	return strings.Join(summary, ", "), nil
}

// verify verifies go.mod integrity
func (g *Go) verify() error {
	if !g.modExists() {
		return fmt.Errorf("go.mod not found")
	}

	_, err := RunCommand("go", "mod", "verify")
	return err
}

// test runs tests
func (g *Go) test() error {
	_, err := RunCommand("go", "test", "./...")
	return err
}

// testRace runs tests with race detector
func (g *Go) testRace() error {
	_, err := RunCommand("go", "test", "-race", "./...")
	return err
}

// getModulePath gets full module path
func (g *Go) getModulePath() (string, error) {
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

// modExists checks if go.mod exists
func (g *Go) modExists() bool {
	_, err := os.Stat("go.mod")
	return err == nil
}

// updateDependents updates modules that depend on the current one
func (g *Go) updateDependents(modulePath, version, searchPath string) (int, error) {
	if searchPath == "" {
		searchPath = ".."
	}

	// Find modules that depend on current
	dependents, err := g.findDependentModules(modulePath, searchPath)
	if err != nil {
		return 0, err
	}

	if len(dependents) == 0 {
		return 0, nil
	}

	// Update each dependent
	updated := 0
	for _, depDir := range dependents {
		if err := g.updateModule(depDir, modulePath, version); err != nil {
			// Log warning?
			// We can't log easily without polluting output.
			// Maybe accumulate errors?
			continue
		}
		updated++
	}

	return updated, nil
}

// findDependentModules searches for modules that have modulePath as dependency
func (g *Go) findDependentModules(modulePath, searchPath string) ([]string, error) {
	var dependents []string

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue despite errors
		}

		// Only go.mod files
		if info.Name() != "go.mod" {
			return nil
		}

		if g.hasDependency(path, modulePath) {
			dependents = append(dependents, filepath.Dir(path))
		}

		return nil
	})

	return dependents, err
}

// hasDependency checks if a go.mod contains a specific dependency
func (g *Go) hasDependency(gomodPath, modulePath string) bool {
	content, err := os.ReadFile(gomodPath)
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Ignore the module declaration of the file itself
		if strings.HasPrefix(line, "module ") {
			if strings.TrimSpace(strings.TrimPrefix(line, "module")) == modulePath {
				return false
			}
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

// updateModule updates a specific module to a new version
func (g *Go) updateModule(moduleDir, dependency, version string) error {
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(moduleDir); err != nil {
		return err
	}

	target := fmt.Sprintf("%s@%s", dependency, version)
	cmd := exec.Command("go", "get", "-u", target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go get failed: %w", err)
	}

	cmd = exec.Command("go", "mod", "tidy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}
