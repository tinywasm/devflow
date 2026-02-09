package devflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Go handler for Go operations
type Go struct {
	rootDir       string
	git           GitClient // Interface for better testing
	log           func(...any)
	backup        *DevBackup
	retryDelay    time.Duration
	retryAttempts int
}

// GoVersion reads the Go version from the go.mod file in the current directory.
// It returns the version string (e.g., "1.18") or an empty string if not found.
func (g *Go) GoVersion() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`^go\s+(\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", nil
}

// NewGo creates a new Go handler and verifies Go installation
func NewGo(gitHandler GitClient) (*Go, error) {
	// Verify go installation
	if _, err := RunCommandSilent("go", "version"); err != nil {
		return nil, fmt.Errorf("go is not installed or not in PATH: %w", err)
	}

	return &Go{
		rootDir:       ".",
		git:           gitHandler,
		backup:        NewDevBackup(),
		log:           func(...any) {}, // default no-op
		retryDelay:    5 * time.Second,
		retryAttempts: 3,
	}, nil
}

// SetRetryConfig sets the retry configuration for network operations
func (g *Go) SetRetryConfig(delay time.Duration, attempts int) {
	g.retryDelay = delay
	g.retryAttempts = attempts
}

// SetRootDir sets the root directory for Go operations
func (g *Go) SetRootDir(path string) {
	g.rootDir = path
}

// SetLog sets the logger function
func (g *Go) SetLog(fn func(...any)) {
	if fn != nil {
		g.log = fn
		if g.git != nil {
			g.git.SetLog(fn)
		}
		if g.backup != nil {
			g.backup.SetLog(fn)
		}
	}
}

// Push executes the complete workflow for Go projects
// Parameters:
//
//	message: Commit message
//	tag: Optional tag
//	skipTests: If true, skips tests
//	skipRace: If true, skips race tests
//	skipDependents: If true, skips updating dependent modules
//	skipBackup: If true, skips backup
//	searchPath: Path to search for dependent modules (default: "..")
func (g *Go) Push(message, tag string, skipTests, skipRace, skipDependents, skipBackup bool, searchPath string) (string, error) {
	// Validate message
	if err := ValidateCommitMessage(message); err != nil {
		return "", err
	}
	message = FormatCommitMessage(message)

	if searchPath == "" {
		searchPath = ".."
	}

	summary := []string{}

	// 1. Verify go.mod
	if err := g.verify(); err != nil {
		return "", fmt.Errorf("go mod verify failed: %w", err)
	}

	// 2. Run tests (if not skipped)
	if !skipTests {
		testSummary, err := g.Test([]string{}, skipRace) // Empty slice = full test suite
		if err != nil {
			return "", fmt.Errorf("tests failed: %w", err)
		}
		summary = append(summary, testSummary)
	} else {
		summary = append(summary, "Tests skipped")
	}

	// 3. Execute git push workflow
	pushResult, err := g.git.Push(message, tag)
	if err != nil {
		return "", fmt.Errorf("push workflow failed: %w", err)
	}
	summary = append(summary, pushResult.Summary)

	// 4. Use the tag that was actually created and pushed
	createdTag := pushResult.Tag
	if createdTag == "" {
		summary = append(summary, "Warning: no tag was created during push")
	}

	// 5. Get module name
	modulePath, err := g.getModulePath()
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: could not get module path: %v", err))
		return strings.Join(summary, ", "), nil
	}

	// 6. Update dependent modules (only if we have a valid tag)
	if !skipDependents && createdTag != "" {
		updateResults, err := g.updateDependents(modulePath, createdTag, searchPath)
		if err != nil {
			summary = append(summary, fmt.Sprintf("Warning: failed to scan dependents: %v", err))
		}
		if len(updateResults) > 0 {
			summary = append(summary, updateResults...)
		}
	}

	// 7. Execute backup (asynchronous, non-blocking)
	if !skipBackup {
		if backupMsg, err := g.backup.Run(); err != nil {
			summary = append(summary, fmt.Sprintf("❌ backup failed to start: %v", err))
		} else if backupMsg != "" {
			summary = append(summary, backupMsg)
		}
	}

	return strings.Join(summary, ", "), nil
}

// UpdateDependentModule updates a dependent module and optionally pushes it
// This is called for each module that depends on the one we just published
// UpdateDependentModule updates a specific dependent module
// It modifies go.mod to require the new version and runs go mod tidy
func (g *Go) UpdateDependentModule(depDir, modulePath, version string) (string, error) {
	depName := filepath.Base(depDir)
	fmt.Printf("📦 Processing dependent: %s\n", depName)

	// 1-2. Load and modify go.mod
	// Since NewGoModFile reads from disk, we pass full path
	// 1-2. Load and modify go.mod
	// Since NewGoModFile reads from disk, we pass full path
	modFile := filepath.Join(depDir, "go.mod")
	gomod := NewGoModHandler()
	gomod.SetRootDir(depDir)
	// No error check needed for creation, but methods will fail if file missing

	// Check/Load explicitly if we want to fail fast?
	// But current flow relied on NewGoModHandler returning error.
	// Since we moved loading to methods, we can't fail fast here unless we call something.
	// But RemoveReplace returns bool.
	// Save() returns error if write fails.
	// Maybe we should just proceed.
	// The original code returned error if file not found.
	// Let's verify file existence manually?
	if _, err := os.Stat(modFile); err != nil {
		return "", fmt.Errorf("failed to load go.mod: %w", err)
	}

	gomod.RemoveReplace(modulePath)

	// 3. Save changes (GoModFile saves to its absolute path)
	if err := gomod.Save(); err != nil {
		return "", fmt.Errorf("failed to save go.mod: %w", err)
	}

	// 4. Smart Update Logic
	currentVer, err := g.GetCurrentVersion(depDir, modulePath)
	if err == nil {
		if CompareVersions(currentVer, version) >= 0 {
			return fmt.Sprintf("already up-to-date (%s)", currentVer), nil
		}
	}

	// 4.1 Run go get WITHOUT -u using explicit directory context
	target := fmt.Sprintf("%s@%s", modulePath, version)

	// Note: We use RunCommandWithRetryInDir here
	if _, err := RunCommandWithRetryInDir(depDir, "go", []string{"get", target}, g.retryAttempts, g.retryDelay); err != nil {
		return "", fmt.Errorf("go get failed after retries: %w", err)
	}

	// 5. Run go mod tidy in the specific directory
	// Note: GoModFile.RunTidy() uses a separate exec.Command with Dir set, but to be consistent
	// and ensure we don't rely on side-effects, we can call it directly or ensure RunTidy works safely.
	// Looking at GoModFile.RunTidy, it uses cmd.Dir = filepath.Dir(m.path), so it IS SAFE.
	// However, let's explicitely use our safe helper if we prefer, OR trust RunTidy.
	// The original code called gomod.RunTidy(). Let's stick to using RunCommandInDir for explicit control if gomod.RunTidy logic is hidden.
	// But gomod.RunTidy IS safe (it takes absolute path from constructor).
	// Let's use RunCommandInDir for consistency with 'go get' above to be 100% sure we control the execution.
	if _, err := RunCommandInDir(depDir, "go", "mod", "tidy"); err != nil {
		return "", fmt.Errorf("go mod tidy failed: %w", err)
	}

	// 6. Check for other replaces
	if gomod.HasOtherReplaces(modulePath) {
		return "updated (other replaces exist, manual push required)", nil
	}

	// 7. Push the dependent module
	// Save current directory and change to depDir for the push
	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := os.Chdir(depDir); err != nil {
		return "", fmt.Errorf("failed to change to dependent directory: %w", err)
	}

	// Ensure we restore the original directory
	defer func() {
		os.Chdir(originalDir)
	}()

	// Create new handlers for the dependent directory
	git, err := NewGit()
	if err != nil {
		return "", fmt.Errorf("git init failed: %w", err)
	}

	depHandler, err := NewGo(git)
	if err != nil {
		return "", fmt.Errorf("go handler init failed: %w", err)
	}

	// Push with skipDependents=true and skipBackup=true to avoid infinite recursion
	commitMsg := fmt.Sprintf("deps: update %s to %s", filepath.Base(modulePath), version)
	_, err = depHandler.Push(commitMsg, "", true, true, true, true, "")
	if err != nil {
		return "", fmt.Errorf("push failed: %w", err)
	}

	return fmt.Sprintf("updated to %s", version), nil
}

// GetCurrentVersion returns the current version of a dependency in a module
func (g *Go) GetCurrentVersion(moduleDir, dependencyPath string) (string, error) {
	// Use go list -m -json dependencyPath directly in moduleDir
	output, err := RunCommandInDir(moduleDir, "go", "list", "-m", "-json", dependencyPath)
	if err != nil {
		return "", err
	}

	var mod struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal([]byte(output), &mod); err != nil {
		return "", err
	}

	return mod.Version, nil
}
