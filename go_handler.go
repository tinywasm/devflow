package devflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tinywasm/gorun"
)

// Go handler for Go operations
type Go struct {
	rootDir        string
	git            GitClient // Interface for better testing
	log            func(...any)
	consoleOutput  func(string) // output for ConsoleFilter (fmt.Println by default)
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
		log:           func(...any) {},                   // default no-op
		consoleOutput: func(s string) { fmt.Println(s) }, // real-time test output
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

// SetConsoleOutput sets the function for console output (used by ConsoleFilter)
func (g *Go) SetConsoleOutput(fn func(string)) {
	g.consoleOutput = fn
}

// GetLog returns the logger function
func (g *Go) GetLog() func(...any) {
	return g.log
}

// GetGit returns the git client
func (g *Go) GetGit() GitClient {
	return g.git
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
//	skipTag: If true, skips tag generation and pushes without tags
//	searchPath: Path to search for dependent modules (default: "..")
func (g *Go) Push(message, tag string, skipTests, skipRace, skipDependents, skipBackup, skipTag bool, searchPath string) (PushResult, error) {
	// Validate message
	if err := ValidateCommitMessage(message); err != nil {
		return PushResult{}, err
	}
	message = FormatCommitMessage(message)

	if searchPath == "" {
		searchPath = ".."
	}

	summary := []string{}

	// UNIVERSAL: If not a Go project, skip Go-specific steps
	if !g.ModExists() {
		var res PushResult
		var err error
		if skipTag {
			committed, _ := g.git.Commit(message)
			pulled, pushErr := g.git.PushWithoutTags()
			err = pushErr
			res.Summary = "✅ Pushed commits"
			if pulled {
				res.Summary = "🔄 Pulled remote changes, " + res.Summary
			}
			if !committed && err == nil {
				res.Summary = "No changes to commit"
			}
		} else {
			res, err = g.git.Push(message, tag)
		}

		if !skipBackup && err == nil {
			if _, backupErr := g.backup.Run(); backupErr != nil {
				res.Summary += fmt.Sprintf(", ❌ backup failed: %v", backupErr)
			}
		}
		return res, err
	}

	// 1. Verify go.mod
	if err := g.Verify(); err != nil {
		return PushResult{}, fmt.Errorf("go mod verify failed: %w", err)
	}

	// 2. Run tests (if not skipped)
	if !skipTests {
		testSummary, err := g.Test([]string{}, skipRace, 0, false, false) // Empty slice = full test suite, 0 = default timeout, false = allow cache, false = runAll
		if err != nil {
			return PushResult{}, fmt.Errorf("tests failed: %w", err)
		}
		summary = append(summary, testSummary)
	} else {
		summary = append(summary, "Tests skipped")
	}

	// 3. Execute git push workflow
	var pushResult PushResult
	var err error

	if skipTag {
		committed, commitErr := g.git.Commit(message)
		if commitErr != nil {
			return PushResult{}, fmt.Errorf("git commit failed: %w", commitErr)
		}
		pulled, pushErr := g.git.PushWithoutTags()
		if pushErr != nil {
			return PushResult{}, fmt.Errorf("push failed: %w", pushErr)
		}
		pushResult.Summary = "✅ Pushed commits"
		if pulled {
			pushResult.Summary = "🔄 Pulled remote changes, " + pushResult.Summary
		}
		if !committed {
			pushResult.Summary = "No changes to commit"
		}
	} else {
		pushResult, err = g.git.Push(message, tag)
		if err != nil {
			return PushResult{}, fmt.Errorf("push workflow failed: %w", err)
		}
	}
	summary = append(summary, pushResult.Summary)

	// 4. Use the tag that was actually created and pushed
	createdTag := pushResult.Tag
	if createdTag == "" && !skipTag {
		summary = append(summary, "Warning: no tag was created during push")
	}

	// 4.5 Install binaries (if cmd exists) — streamed to console, not summary
	if createdTag != "" {
		if err := g.Install(createdTag); err != nil {
			summary = append(summary, fmt.Sprintf("Warning: install failed: %v", err))
		}
	}

	// 5. Get module name
	modulePath, err := g.GetModulePath()
	if err != nil {
		summary = append(summary, fmt.Sprintf("Warning: could not get module path: %v", err))
		return PushResult{Summary: strings.Join(summary, ", "), Tag: createdTag}, nil
	}

	// 6. Update dependent modules (only if we have a valid tag)
	if !skipDependents && createdTag != "" {
		if err := g.UpdateDependents(modulePath, createdTag, searchPath); err != nil {
			summary = append(summary, fmt.Sprintf("Warning: failed to scan dependents: %v", err))
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

	return PushResult{Summary: strings.Join(summary, ", "), Tag: createdTag}, nil
}

// Publish satisfies the Publisher interface
func (g *Go) Publish(message, tag string, skipTests, skipRace, skipDependents, skipBackup, skipTag bool) (PushResult, error) {
	return g.Push(message, tag, skipTests, skipRace, skipDependents, skipBackup, skipTag, "..")
}

// UpdateDependentModule updates a dependent module and optionally pushes it
// This is called for each module that depends on the one we just published
// UpdateDependentModule updates a specific dependent module
// It modifies go.mod to require the new version and runs go mod tidy
func (g *Go) UpdateDependentModule(depDir, modulePath, version string) (string, error) {
	depName := filepath.Base(depDir)

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
		g.consoleOutput(fmt.Sprintf("📦 %s → ⏭ skip push (other replaces exist)", depName))
		return "updated (other replaces exist, manual push required)", nil
	}

	// 7. Push the dependent module
	// Create new handlers for the dependent directory
	git, err := NewGit()
	if err != nil {
		return "", fmt.Errorf("git init failed: %w", err)
	}
	git.SetRootDir(depDir)

	depHandler, err := NewGo(git)
	if err != nil {
		return "", fmt.Errorf("go handler init failed: %w", err)
	}
	depHandler.SetRootDir(depDir)

	// Push with skipDependents=true and skipBackup=true to avoid infinite recursion
	// We pass the depDir as searchPath just in case, though it's not used here since skipDependents is true
	commitMsg := fmt.Sprintf("deps: update %s to %s", filepath.Base(modulePath), version)
	_, err = depHandler.Push(commitMsg, "", false, true, true, true, false, "")
	if err != nil {
		g.consoleOutput(fmt.Sprintf("📦 %s → ❌ tests failed", depName))
		return "", fmt.Errorf("push failed: %w", err)
	}

	g.consoleOutput(fmt.Sprintf("📦 %s → ✅ updated to %s", depName, version))
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

// Install builds and installs all commands in the cmd/ directory
// It injects the version using ldflags if provided
func (g *Go) Install(version string) error {
	cmdDir := filepath.Join(g.rootDir, "cmd")
	if _, err := os.Stat(cmdDir); os.IsNotExist(err) {
		return nil // No cmd directory, skip silently
	}

	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return fmt.Errorf("failed to read cmd directory: %w", err)
	}

	var commands []string
	for _, entry := range entries {
		if entry.IsDir() {
			commands = append(commands, entry.Name())
		}
	}

	if len(commands) == 0 {
		return nil // No commands found
	}

	ldflags := ""
	actualVersion := version
	if actualVersion == "" && g.git != nil {
		if tag, err := g.git.GetLatestTag(); err == nil && tag != "" {
			actualVersion = tag
		}
	}

	if actualVersion != "" {
		ldflags = fmt.Sprintf("-ldflags=-X main.Version=%s", actualVersion)
	}

	for _, cmd := range commands {
		_ = gorun.StopApp(cmd) // Kill any running instance before install
		pkg := "./cmd/" + cmd
		args := []string{"install"}
		if ldflags != "" {
			args = append(args, ldflags)
		}
		args = append(args, pkg)

		if _, err := RunCommandInDir(g.rootDir, "go", args...); err != nil {
			return fmt.Errorf("failed to install %s: %w", cmd, err)
		}
		g.consoleOutput(fmt.Sprintf("✅ %s", cmd))
	}

	return nil
}
