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
	rootDir string
	git     *Git
	log     func(...any)
	backup  *DevBackup
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
func NewGo(gitHandler *Git) (*Go, error) {
	// Verify go installation
	if _, err := RunCommandSilent("go", "version"); err != nil {
		return nil, fmt.Errorf("go is not installed or not in PATH: %w", err)
	}

	return &Go{
		rootDir: ".",
		git:     gitHandler,
		backup:  NewDevBackup(),
		log:     func(...any) {}, // default no-op
	}, nil
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
		testSummary, err := g.Test() // quiet mode
		if err != nil {
			return "", fmt.Errorf("tests failed: %w", err)
		}
		summary = append(summary, testSummary)
	} else {
		summary = append(summary, "Tests skipped")
	}

	// 3. Execute git push workflow
	pushSummary, err := g.git.Push(message, tag)
	if err != nil {
		return "", fmt.Errorf("push workflow failed: %w", err)
	}
	summary = append(summary, pushSummary)

	// 4. Get created tag
	latestTag, err := g.git.GetLatestTag()
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: could not get latest tag: %v", err))
		// Not fatal error
	}

	// 5. Get module name
	modulePath, err := g.getModulePath()
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: could not get module path: %v", err))
		return strings.Join(summary, ", "), nil
	}

	// 6. Update dependent modules
	if !skipDependents {
		updateResults, err := g.updateDependents(modulePath, latestTag, searchPath)
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
func (g *Go) UpdateDependentModule(depDir, modulePath, version string) (string, error) {
	originalDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(depDir); err != nil {
		return "", err
	}

	depName := filepath.Base(depDir)
	fmt.Printf("\n📦 Processing dependent: %s\n", depName)

	// 1-2. Load and modify go.mod
	gomod, err := NewGoModFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("failed to load go.mod: %w", err)
	}

	gomod.RemoveReplace(modulePath)

	// 3. Save changes
	if err := gomod.Save(); err != nil {
		return "", fmt.Errorf("failed to save go.mod: %w", err)
	}

	// 4. Smart Update Logic
	// Check current version first to avoid unnecessary updates and network calls
	currentVer, err := g.GetCurrentVersion(depDir, modulePath)
	if err == nil {
		// If current version is greater or equal to target, skip
		if CompareVersions(currentVer, version) >= 0 {
			return fmt.Sprintf("already up-to-date (%s)", currentVer), nil
		}
	}

	// Run go get WITHOUT -u to avoid breaking transitive dependencies like goflare
	// We only want to update the specific module we strictly require.
	target := fmt.Sprintf("%s@%s", modulePath, version)
	retryDelay := 5 * time.Second
	// Note: Removed "-u" flag here
	if _, err := RunCommandWithRetry("go", []string{"get", target}, 3, retryDelay); err != nil {
		return "", fmt.Errorf("go get failed after retries: %w", err)
	}

	// 5. Run go mod tidy
	if err := gomod.RunTidy(); err != nil {
		return "", fmt.Errorf("go mod tidy failed: %w", err)
	}

	// 6. Check for other replaces
	if gomod.HasOtherReplaces(modulePath) {
		return "updated (other replaces exist, manual push required)", nil
	}

	// 7. Push with skipDependents=true, skipBackup=true
	git, err := NewGit()
	if err != nil {
		return "", fmt.Errorf("git init failed: %w", err)
	}
	depHandler, err := NewGo(git)
	if err != nil {
		return "", fmt.Errorf("go handler init failed: %w", err)
	}

	// Extract package name from module path (last segment)
	parts := strings.Split(modulePath, "/")
	pkgName := parts[len(parts)-1]
	message := fmt.Sprintf("deps: update %s to %s", pkgName, version)

	// Recursive call skipping dependents and backup
	summary, err := depHandler.Push(message, "", false, false, true, true, "")
	if err != nil {
		return "", fmt.Errorf("push failed: %w", err)
	}

	return fmt.Sprintf("pushed (%s)", summary), nil
}

// GetCurrentVersion returns the current version of a dependency in a module
func (g *Go) GetCurrentVersion(moduleDir, dependencyPath string) (string, error) {
	originalDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(moduleDir); err != nil {
		return "", err
	}

	// Use go list -m -json dependencyPath
	output, err := RunCommand("go", "list", "-m", "-json", dependencyPath)
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
