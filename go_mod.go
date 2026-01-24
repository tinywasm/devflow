package devflow

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GoModHandler represents a parsed go.mod file and handles file events
type GoModHandler struct {
	lines    []string // all lines of the file
	modified bool     // track if changes were made

	// Handler fields
	rootDir       string
	watcher       FolderWatcher
	currentPaths  map[string]string // modulePath -> localPath
	log           func(messages ...any)
	knownReplaces map[string]string
}

// ReplaceEntry represents a local replace directive found in go.mod
type ReplaceEntry struct {
	ModulePath string // The module being replaced
	LocalPath  string // The local path replacement
}

// NewGoModHandler reads and parses a go.mod file or returns an empty handler if path is empty
func NewGoModHandler() *GoModHandler {
	return &GoModHandler{
		rootDir:       ".",
		currentPaths:  make(map[string]string),
		knownReplaces: make(map[string]string),
		log:           func(messages ...any) {},
	}
}

func (g *GoModHandler) load() error {
	gomodPath := filepath.Join(g.rootDir, "go.mod")
	content, err := os.ReadFile(gomodPath)
	if err != nil {
		return err
	}

	g.lines = strings.Split(string(content), "\n")
	return nil
}

// RemoveReplace removes a replace directive for the given module
// Returns true if a replace was found and removed
func (m *GoModHandler) RemoveReplace(modulePath string) bool {
	// check if loaded
	if len(m.lines) == 0 {
		if err := m.load(); err != nil {
			return false
		}
	}

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

// GetLocalReplacePaths returns absolute paths from local replace directives.
// Relative paths are resolved starting from the directory containing go.mod.
func (m *GoModHandler) GetLocalReplacePaths() ([]ReplaceEntry, error) {
	// check if loaded
	if len(m.lines) == 0 {
		if err := m.load(); err != nil {
			return nil, err
		}
	}

	var entries []ReplaceEntry
	inReplaceBlock := false
	rootDir := m.rootDir

	for _, line := range m.lines {
		trimmed := strings.TrimSpace(line)

		// Detect start/end of replace block
		if strings.HasPrefix(trimmed, "replace (") {
			inReplaceBlock = true
			continue
		}
		if inReplaceBlock && trimmed == ")" {
			inReplaceBlock = false
			continue
		}

		if strings.HasPrefix(trimmed, "replace ") || inReplaceBlock {
			// Extract part after "replace " if not in block
			lineContent := trimmed
			if !inReplaceBlock {
				lineContent = strings.TrimPrefix(trimmed, "replace ")
			}

			// Format is usually: module => path
			parts := strings.Split(lineContent, "=>")
			if len(parts) != 2 {
				continue
			}

			modPath := strings.TrimSpace(parts[0])
			localPath := strings.TrimSpace(parts[1])

			// Clean up comments if any
			if idx := strings.Index(localPath, "//"); idx != -1 {
				localPath = strings.TrimSpace(localPath[:idx])
			}

			// Check if localPath is actually a local path or a versioned module.
			// Local paths in go.mod MUST start with ./ or ../ or be absolute.
			isLocal := strings.HasPrefix(localPath, ".") || strings.HasPrefix(localPath, "/")
			if !isLocal {
				continue
			}

			// Resolve to absolute path
			absPath := localPath
			if !filepath.IsAbs(localPath) {
				absPath = filepath.Join(rootDir, localPath)
			}
			absPath, _ = filepath.Abs(absPath)

			entries = append(entries, ReplaceEntry{
				ModulePath: modPath,
				LocalPath:  absPath,
			})
		}
	}

	return entries, nil
}

// HasOtherReplaces returns true if there are replace directives
// other than the specified module
func (m *GoModHandler) HasOtherReplaces(exceptModule string) bool {
	// check if loaded
	if len(m.lines) == 0 {
		if err := m.load(); err != nil {
			return false
		}
	}

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
func (m *GoModHandler) Save() error {
	if !m.modified {
		return nil
	}

	content := strings.Join(m.lines, "\n")
	return os.WriteFile(filepath.Join(m.rootDir, "go.mod"), []byte(content), 0644)
}

// RunTidy executes 'go mod tidy' in the directory of the go.mod file
func (m *GoModHandler) RunTidy() error {
	dir := m.rootDir
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	_, err := cmd.CombinedOutput()
	return err
}

func (g *GoModHandler) SetRootDir(path string) {
	g.rootDir = path
}

func (g *GoModHandler) SetFolderWatcher(watcher FolderWatcher) {
	g.watcher = watcher
}

func (g *GoModHandler) SetLog(fn func(messages ...any)) {
	g.log = fn
}

func (g *GoModHandler) Name() string {
	return "GOMOD"
}

func (g *GoModHandler) MainInputFileRelativePath() string {
	return "go.mod"
}

func (g *GoModHandler) SupportedExtensions() []string {
	return []string{".mod"}
}

func (g *GoModHandler) UnobservedFiles() []string {
	return nil
}

// NewFileEvent handles changes to go.mod
func (g *GoModHandler) NewFileEvent(fileName, extension, filePath, event string) error {
	// Only care about go.mod in the root directory
	if !strings.HasSuffix(filePath, "go.mod") {
		return nil
	}

	// Double check it's the root go.mod if rootDir is set
	if g.rootDir != "" {
		absFilePath, _ := filepath.Abs(filePath)
		absGoMod := filepath.Join(g.rootDir, "go.mod")
		if absFilePath != absGoMod {
			return nil
		}
	}

	// Refresh lines from file
	content, err := os.ReadFile(filePath)
	if err != nil {
		g.log("Error reading go.mod:", err)
		return err
	}
	g.lines = strings.Split(string(content), "\n")
	g.modified = false

	entries, err := g.GetLocalReplacePaths()
	if err != nil {
		g.log("Error getting replace paths:", err)
		return err
	}

	g.reconcilePaths(entries)
	return nil
}

func (g *GoModHandler) reconcilePaths(entries []ReplaceEntry) {
	newMap := make(map[string]string)
	for _, entry := range entries {
		newMap[entry.ModulePath] = entry.LocalPath
	}

	// If watcher is not set, we can't do anything but update state
	if g.watcher == nil {
		g.currentPaths = newMap
		return
	}

	// Add new paths
	for mod, path := range newMap {
		if _, exists := g.currentPaths[mod]; !exists {
			g.log("GoModHandler: Watching external module:", path)
			if err := g.watcher.AddDirectoryToWatcher(path); err != nil {
				g.log("Frontend Error: Failed to watch external module:", path, err)
			}
		}
	}

	// Remove old paths
	for mod, path := range g.currentPaths {
		if _, exists := newMap[mod]; !exists {
			g.log("GoModHandler: Stop watching external module:", path)
			if err := g.watcher.RemoveDirectoryFromWatcher(path); err != nil {
				g.log("Frontend Error: Failed to remove watch for external module:", path, err)
			}
		}
	}

	g.currentPaths = newMap
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

// FindProjectRoot looks for go.mod in startDir or its immediate parent.
// Returns the absolute path to the directory containing go.mod, or an empty string and error if not found.
func FindProjectRoot(startDir string) (string, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	// Check current directory
	if _, err := os.Stat(filepath.Join(absStart, "go.mod")); err == nil {
		return absStart, nil
	}

	// Check parent directory
	parent := filepath.Dir(absStart)
	if parent != absStart { // Avoid checking same dir if at root
		if _, err := os.Stat(filepath.Join(parent, "go.mod")); err == nil {
			return parent, nil
		}
	}

	return "", fmt.Errorf("could not find go.mod in %s or parent", absStart)
}

// verify verifies go.mod integrity
func (g *Go) verify() error {
	if !g.modExists() {
		return fmt.Errorf("go.mod not found")
	}

	_, err := RunCommand("go", "mod", "verify")
	return err
}

// WaitForVersionAvailable waits for a module version to be available on Go proxy
func (g *Go) WaitForVersionAvailable(modulePath, version string) error {
	target := fmt.Sprintf("%s@%s", modulePath, version)
	maxRetries := 3
	delay := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		_, err := RunCommandSilent("go", "list", "-m", target)
		if err == nil {
			return nil
		}
		if i < maxRetries-1 {
			fmt.Printf("⏳ Waiting for %s (attempt %d/%d)...\n", version, i+1, maxRetries)
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("version %s not available after %d attempts", version, maxRetries)
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

	// Wait for version to be available before updating any dependents
	if err := g.WaitForVersionAvailable(modulePath, version); err != nil {
		return []string{fmt.Sprintf("⏳ %s", err)}, nil
	}

	// Update each dependent sequentially to avoid os.Chdir race conditions
	var results []string
	for _, depDir := range dependents {
		depName := filepath.Base(depDir)
		result, err := g.UpdateDependentModule(depDir, modulePath, version)
		if err != nil {
			results = append(results, fmt.Sprintf("❌ %s: %v", depName, err))
		} else {
			results = append(results, fmt.Sprintf("✅ %s: %s", depName, result))
		}
	}

	fmt.Println()
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
