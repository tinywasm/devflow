package devflow

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoModFile represents a parsed go.mod file for efficient operations
type GoModFile struct {
	path     string   // absolute path to go.mod
	lines    []string // all lines of the file
	modified bool     // track if changes were made
}

// NewGoModFile reads and parses a go.mod file
func NewGoModFile(gomodPath string) (*GoModFile, error) {
	content, err := os.ReadFile(gomodPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	return &GoModFile{
		path:  gomodPath,
		lines: lines,
	}, nil
}

// RemoveReplace removes a replace directive for the given module
// Returns true if a replace was found and removed
func (m *GoModFile) RemoveReplace(modulePath string) bool {
	originalCount := len(m.lines)
	var newLines []string
	inReplaceBlock := false
	removed := false

	for _, line := range m.lines {
		trimmed := strings.TrimSpace(line)

		// Detect start/end of replace block
		if strings.HasPrefix(trimmed, "replace (") {
			inReplaceBlock = true
			newLines = append(newLines, line)
			continue
		}
		if inReplaceBlock && trimmed == ")" {
			inReplaceBlock = false
			// Check if we just emptied the block (last line was "replace (")
			if len(newLines) > 0 && strings.HasPrefix(strings.TrimSpace(newLines[len(newLines)-1]), "replace (") {
				newLines = newLines[:len(newLines)-1] // remove "replace ("
				removed = true
				continue
			}
			newLines = append(newLines, line)
			continue
		}

		// Check for the module in replace
		if (strings.HasPrefix(trimmed, "replace ") || inReplaceBlock) && strings.Contains(trimmed, modulePath) {
			removed = true
			continue // skip this line
		}

		newLines = append(newLines, line)
	}

	if removed || len(newLines) != originalCount {
		m.lines = newLines
		m.modified = true
		return true
	}

	return false
}

// HasOtherReplaces returns true if there are replace directives
// other than the specified module
func (m *GoModFile) HasOtherReplaces(exceptModule string) bool {
	inReplaceBlock := false
	for _, line := range m.lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "replace (") {
			inReplaceBlock = true
			continue
		}
		if inReplaceBlock && trimmed == ")" {
			inReplaceBlock = false
			continue
		}

		if (strings.HasPrefix(trimmed, "replace ") || inReplaceBlock) && trimmed != "" {
			if exceptModule != "" && strings.Contains(trimmed, exceptModule) {
				continue
			}
			return true
		}
	}
	return false
}

// Save writes changes back to the file if modified
func (m *GoModFile) Save() error {
	if !m.modified {
		return nil
	}

	content := strings.Join(m.lines, "\n")
	return os.WriteFile(m.path, []byte(content), 0644)
}

// RunTidy executes 'go mod tidy' in the directory of the go.mod file
func (m *GoModFile) RunTidy() error {
	dir := filepath.Dir(m.path)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	_, err := cmd.CombinedOutput()
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
		result, err := g.UpdateDependentModule(depDir, modulePath, version)
		if err != nil {
			results = append(results, fmt.Sprintf("❌ %s: %v", depName, err))
			continue
		}
		results = append(results, fmt.Sprintf("✅ %s: %s", depName, result))
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
