package devflow

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	_, err := os.Stat(filepath.Join(g.rootDir, "go.mod"))
	return err == nil
}

// ModExistsInCurrentOrParent checks if go.mod exists in the rootDir or one directory up.
func (g *Go) ModExistsInCurrentOrParent() bool {
	// Check in rootDir
	if g.modExists() {
		return true
	}
	// Check in parent
	parentDir := filepath.Dir(g.rootDir)
	if parentDir != g.rootDir { // Avoid infinite loop at system root
		_, err := os.Stat(filepath.Join(parentDir, "go.mod"))
		return err == nil
	}
	return false
}

// verify verifies go.mod integrity
func (g *Go) verify() error {
	if !g.modExists() {
		return fmt.Errorf("go.mod not found")
	}

	_, err := RunCommand("go", "mod", "verify")
	return err
}

// updateDependents updates modules that depend on the current one
func (g *Go) updateDependents(modulePath, version, searchPath string) ([]string, error) {
	if searchPath == "" {
		searchPath = ".."
	}

	// Find modules that depend on current
	dependents, err := g.findDependentModules(modulePath, searchPath)
	if err != nil {
		return nil, err
	}

	if len(dependents) == 0 {
		return nil, nil
	}

	// Update each dependent
	var results []string
	for _, depDir := range dependents {
		depName := filepath.Base(depDir)
		if err := g.updateModule(depDir, modulePath, version); err != nil {
			results = append(results, fmt.Sprintf("❌ Failed to update %s: %v", depName, err))
			continue
		}
		results = append(results, fmt.Sprintf("✅ Updated %s", depName))
	}

	return results, nil
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
	_, err = RunCommand("go", "get", "-u", target)
	if err != nil {
		return fmt.Errorf("go get failed: %w", err)
	}

	_, err = RunCommand("go", "mod", "tidy")
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}

// ModInit initializes a new go module
func (g *Go) ModInit(modulePath, targetDir string) error {
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(targetDir); err != nil {
		return err
	}

	_, err = RunCommand("go", "mod", "init", modulePath)
	return err
}

// DetectGoExecutable returns the path to the go executable
func (g *Go) DetectGoExecutable() (string, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	return path, nil
}
