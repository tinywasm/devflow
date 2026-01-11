package devflow

import (
	"fmt"
	"os"
	"regexp"
	"strings"
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
//	searchPath: Path to search for dependent modules (default: "..")
func (g *Go) Push(message, tag string, skipTests, skipRace bool, searchPath string) (string, error) {
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
	updateResults, err := g.updateDependents(modulePath, latestTag, searchPath)
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: failed to scan dependents: %v", err))
		// Not fatal error
	}
	if len(updateResults) > 0 {
		summary = append(summary, updateResults...)
	}

	// 7. Execute backup (asynchronous, non-blocking)
	if backupMsg, err := g.backup.Run(); err != nil {
		summary = append(summary, fmt.Sprintf("❌ backup failed to start: %v", err))
	} else if backupMsg != "" {
		summary = append(summary, backupMsg)
	}

	return strings.Join(summary, ", "), nil
}
