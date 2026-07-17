package devflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tinywasm/command"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tinywasm/gorun"
)

// CrossTarget represents a compilation target platform
type CrossTarget struct{ GOOS, GOARCH string }

// Go handler for Go operations
type Go struct {
	rootDir               string
	git                   GitClient // Interface for better testing
	log                   func(...any)
	consoleOutput         func(string) // output for ConsoleFilter (fmt.Println by default)
	backup                BackupRunner
	retryDelay            time.Duration
	retryAttempts         int
	crossCompileFn        func(tmpDir string, cmds []string, targets []CrossTarget, repoDir string) ([]string, error)
	extraPublishObjectors []PublishObjector
	useTinygo             bool
}

// GoVersion reads the Go version from the go.mod file in the current directory.
// It returns the version string (e.g., "1.18") or an empty string if not found.
func (g *Go) GoVersion() (string, error) {
	data, err := os.ReadFile(filepath.Join(g.rootDir, "go.mod"))
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", nil
}

// CodejobPhaseOf reports the current codejob phase for the given directory.
func CodejobPhaseOf(dir string) CodejobPhase {
	meta, err := ReadPlanMeta(filepath.Join(dir, DefaultIssuePromptPath))
	if err != nil {
		return ""
	}
	switch strings.ToLower(meta.Status) {
	case "running":
		return PhaseRunning
	case "reviewing":
		return PhaseRunning
	case "review":
		return PhaseReview
	default:
		return ""
	}
}

// WorkTreeDirtyBeyond returns true if the git worktree has changes beyond the allowed files.
// It ignores .env and .gitignore files automatically.
func WorkTreeDirtyBeyond(git GitClient, allowed ...string) (bool, error) {
	out, err := git.StatusPorcelain()
	if err != nil {
		return false, err
	}

	if out == "" {
		return false, nil
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue
		}

		// git status --porcelain output:
		// XY PATH
		// XY is 2 characters
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		file := strings.TrimSpace(parts[1])
		// If the file is quoted, unquote it (simplistic version)
		file = strings.Trim(file, "\"")
		if file == "" {
			continue
		}

		// Always ignore .env and .gitignore
		if file == ".env" || file == ".gitignore" {
			continue
		}

		// Check if it's in the allowed list
		isAllowed := false
		for _, a := range allowed {
			if file == a {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			return true, nil
		}
	}

	return false, nil
}

// CascadeStatus constants represent the possible outcomes of a module update in a cascade.
const (
	CascadeStatusPublished = "published"
	CascadeStatusDepsOnly  = "deps only"
	CascadeStatusSkipped   = "skipped"
	CascadeStatusFailed    = "failed"
)

// NewGo creates a new Go handler and verifies Go installation
func NewGo(gitHandler GitClient) (*Go, error) {
	// Verify go installation
	if _, err := command.Run("go", "version"); err != nil {
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

// SetBackup replaces the backup runner (used in tests to inject a mock).
func (g *Go) SetBackup(b BackupRunner) {
	g.backup = b
}

// SetConsoleOutput sets the function for console output (used by ConsoleFilter)
func (g *Go) SetConsoleOutput(fn func(string)) {
	g.consoleOutput = fn
}

// SetPublishObjectors replaces the extra publish objectors.
func (g *Go) SetPublishObjectors(objs ...PublishObjector) { g.extraPublishObjectors = objs }

// AddPublishObjector appends an extra publish objector.
func (g *Go) AddPublishObjector(obj PublishObjector) {
	g.extraPublishObjectors = append(g.extraPublishObjectors, obj)
}

// SetCrossCompileFn sets a custom cross-compile function for testing
func (g *Go) SetCrossCompileFn(fn func(tmpDir string, cmds []string, targets []CrossTarget, repoDir string) ([]string, error)) {
	g.crossCompileFn = fn
}

// GetLog returns the logger function
func (g *Go) GetLog() func(...any) {
	return g.log
}

// GetGit returns the git client
func (g *Go) GetGit() GitClient {
	return g.git
}

// ErrPushBlockedActiveCodejob is returned by Push when the repo has an active codejob
// session: publishing would move the base branch under the agent.
const ErrPushBlockedActiveCodejob = "gopush blocked: active codejob session (CODEJOB in .env) — the repo is under agent control; run 'codejob' to check status and close the loop before publishing"

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
func (g *Go) Push(message, tag string, skipTests, skipRace, skipDependents, skipBackup, skipTag, skipVerify bool, searchPath string) (PushResult, error) {
	// Validate message
	if err := ValidateCommitMessage(message); err != nil {
		return PushResult{}, err
	}
	message = FormatCommitMessage(message)

	// Block push only during 'running' phase. During 'review' phase,
	// MergeAndPublish calls Push to close the loop, so blocking 'review'
	// would self-block codejob.
	if CodejobPhaseOf(g.rootDir) == PhaseRunning {
		return PushResult{}, errors.New(ErrPushBlockedActiveCodejob)
	}

	if searchPath == "" {
		searchPath = ".."
	}

	summary := []string{}

	// 0. Early exit if nothing to push
	hasPending, _ := g.git.HasPendingChanges()
	if !hasPending {
		return PushResult{Summary: "Nothing to push"}, nil
	}

	// UNIVERSAL: If not a Go project, skip Go-specific steps
	if !g.ModExists() {
		var res PushResult
		var err error
		if skipTag {
			if err := g.git.Add(); err != nil {
				return PushResult{}, fmt.Errorf("git add failed: %w", err)
			}
			committed, _ := g.git.Commit(message)
			pulled, pushErr := g.git.PushWithoutTags()
			err = pushErr
			res.Summary = "Pushed ✅"
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

	// 1. Verify go.mod (skip when dispatching to an agent that will fix the repo)
	if !skipVerify {
		if err := g.Verify(); err != nil {
			return PushResult{}, fmt.Errorf("go mod verify failed: %w", err)
		}
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

	// 3. Prepare internal submodules and execute git push workflow
	var pushResult PushResult
	var err error

	modulePath, _ := g.GetModulePath()

	if skipTag {
		if err := g.git.Add(); err != nil {
			return PushResult{}, fmt.Errorf("git add failed: %w", err)
		}
		committed, commitErr := g.git.Commit(message)
		if commitErr != nil {
			return PushResult{}, fmt.Errorf("git commit failed: %w", commitErr)
		}
		pulled, pushErr := g.git.PushWithoutTags()
		if pushErr != nil {
			return PushResult{}, fmt.Errorf("push failed: %w", pushErr)
		}
		pushResult.Summary = "Pushed ✅"
		if pulled {
			pushResult.Summary = "🔄 Pulled remote changes, " + pushResult.Summary
		}
		if !committed {
			pushResult.Summary = "No changes to commit"
		}
	} else {
		// Hoist tag computation so we can sync internal submodules BEFORE commit
		nextTag := tag
		if nextTag == "" && g.git != nil {
			var err error
			nextTag, err = g.git.GenerateNextTag()
			if err != nil {
				g.log("Warning: could not generate next tag for submodule sync:", err)
			}
		}

		if nextTag != "" && modulePath != "" {
			if err := g.syncInternalSubmodules(modulePath, nextTag); err != nil {
				g.log("Warning: failed to sync internal submodules:", err)
			}
		}

		// Phase 2: Append shortstat to commit message
		if g.git != nil {
			if stat, err := g.git.DiffShortStat(); err == nil && stat != "" {
				message = message + "\n\n" + stat
			}
		}

		pushResult, err = g.git.Push(message, tag)
		if err != nil {
			return PushResult{}, fmt.Errorf("push workflow failed: %w", err)
		}
	}
	summary = append(summary, pushResult.Summary)

	// 4. Use the tag that was actually created and pushed
	createdTag := pushResult.Tag

	// 4.5 Install binaries (if cmd exists) — streamed to console, not summary
	if createdTag != "" {
		if err := g.Install(createdTag); err != nil {
			summary = append(summary, fmt.Sprintf("Warning: install failed: %v", err))
		}
	}

	// 5. Get module name
	modulePath, err = g.GetModulePath()
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
func (g *Go) Publish(message, tag string, skipTests, skipRace, skipDependents, skipBackup, skipTag, skipVerify bool) (PushResult, error) {
	return g.Push(message, tag, skipTests, skipRace, skipDependents, skipBackup, skipTag, skipVerify, "..")
}

// UpdateDependentModule updates a dependent module and optionally pushes it
// This is called for each module that depends on the one we just published
// UpdateDependentModule updates a specific dependent module with multiple bumps
// It modifies go.mod to require the new versions and runs go mod tidy
// reportFail prints why a dependent could not be updated and returns the error.
// Every failing exit path of UpdateDependentModule must go through here: a
// dependent that reports nothing is indistinguishable from one that was never
// attempted, which makes the "Updating N dependents" count a lie.
func (g *Go) reportFail(depName string, err error) (CascadeOutcome, error) {
	g.consoleOutput(fmt.Sprintf("📦 %s → ❌ %v", depName, err))
	return CascadeOutcome{}, err
}

func (g *Go) UpdateDependentModule(depDir string, bumps []DepBump, rootCause string) (CascadeOutcome, error) {
	depName := filepath.Base(depDir)

	// 1. Check if go.mod exists
	modFile := filepath.Join(depDir, "go.mod")
	if _, err := os.Stat(modFile); err != nil {
		return g.reportFail(depName, fmt.Errorf("failed to load go.mod: %w", err))
	}

	// 2. Build objectors and RESOLVE ACTION BEFORE mutation
	gomod := NewGoModHandler()
	gomod.SetRootDir(depDir)

	git, err := NewGit()
	if err != nil {
		return g.reportFail(depName, fmt.Errorf("git init failed: %w", err))
	}
	git.SetRootDir(depDir)

	var modulePaths []string
	for _, b := range bumps {
		modulePaths = append(modulePaths, b.ModulePath)
	}

	objectors := append([]PublishObjector{gomod, git, CodeJob{}}, g.extraPublishObjectors...)
	ctx := PublishContext{RepoDir: depDir, ModulePaths: modulePaths}
	action, reason := ResolvePublishAction(objectors, ctx)

	if action == ActionSkip {
		g.consoleOutput(fmt.Sprintf("📦 %s → skip (%s) ⏭", depName, reason))
		return CascadeOutcome{Status: CascadeStatusSkipped, Reason: reason}, nil
	}

	// 3. Smart Revert Logic
	success := false
	defer func() {
		if !success {
			command.RunInDir(depDir, "git", "checkout", "--", "go.mod", "go.sum")
		}
	}()

	// 4. Check if already up-to-date AND no replace to remove
	anyChange := false
	for _, bump := range bumps {
		canRemove := gomod.RemoveReplace(bump.ModulePath)
		currentVer, err := g.GetCurrentVersion(depDir, bump.ModulePath)
		if err == nil {
			if CompareVersions(currentVer, bump.NewVersion) < 0 || canRemove {
				anyChange = true
			}
		} else {
			anyChange = true // can't determine version, assume change needed
		}
	}

	if !anyChange {
		success = true // no mutation happened
		const reason = "already up-to-date"
		g.consoleOutput(fmt.Sprintf("📦 %s → skip (%s) ⏭", depName, reason))
		return CascadeOutcome{Status: CascadeStatusSkipped, Reason: reason}, nil
	}

	// 5. Mutate
	if gomod.Modified {
		if err := gomod.Save(); err != nil {
			return g.reportFail(depName, fmt.Errorf("failed to save go.mod: %w", err))
		}
	}

	for _, bump := range bumps {
		target := fmt.Sprintf("%s@%s", bump.ModulePath, bump.NewVersion)
		if _, err := command.RunWithRetry(depDir, "go", []string{"get", target}, g.retryAttempts, g.retryDelay); err != nil {
			return g.reportFail(depName, fmt.Errorf("go get failed after retries: %w", err))
		}
	}

	if output, err := command.RunInDir(depDir, "go", "mod", "tidy"); err != nil {
		return g.reportFail(depName, fmt.Errorf("go mod tidy failed: %s", extractFirstFailure(output)))
	}

	_, _ = command.RunInDir(depDir, "go", "generate", "./...")

	// 6. gotest (gate)
	if output, err := command.RunInDir(depDir, "gotest", "-t", "60", "-no-cache"); err != nil {
		cause := extractFirstFailure(output)
		g.consoleOutput(fmt.Sprintf("📦 %s → %s ❌", depName, cause))
		return CascadeOutcome{}, fmt.Errorf("tests failed: %w", err)
	}

	// 7. Push using pre-resolved action
	depHandler, err := NewGo(git)
	if err != nil {
		return g.reportFail(depName, fmt.Errorf("go handler init failed: %w", err))
	}
	depHandler.SetRootDir(depDir)

	commitMsg := BuildDepsCommitMessage(bumps, rootCause)

	if action == ActionDepsOnly {
		committed, err := git.CommitPaths(commitMsg, "go.mod", "go.sum")
		if err != nil {
			return g.reportFail(depName, fmt.Errorf("deps-only commit failed: %w", err))
		}
		if committed {
			if _, err := git.PushWithoutTags(); err != nil {
				return g.reportFail(depName, fmt.Errorf("deps-only push failed: %w", err))
			}
		}
		g.consoleOutput(fmt.Sprintf("📦 %s → %s (%s) ⚠", depName, CascadeStatusDepsOnly, reason))
		success = true
		return CascadeOutcome{Status: CascadeStatusDepsOnly, Reason: reason}, nil
	}

	// Clean tree: full flow (skipTag=false)
	pushRes, err := depHandler.Push(commitMsg, "", true, true, true, true, false, false, "")
	if err != nil {
		g.consoleOutput(fmt.Sprintf("📦 %s → ❌ push failed", depName))
		return CascadeOutcome{}, fmt.Errorf("push failed: %w", err)
	}

	g.consoleOutput(fmt.Sprintf("📦 %s → updated ✅", depName))
	success = true
	return CascadeOutcome{Status: CascadeStatusPublished, Version: pushRes.Tag}, nil
}

// GetCurrentVersion returns the current version of a dependency in a module
func (g *Go) GetCurrentVersion(moduleDir, dependencyPath string) (string, error) {
	// Use go list -m -json dependencyPath directly in moduleDir
	output, err := command.RunInDir(moduleDir, "go", "list", "-m", "-json", dependencyPath)
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

// extractFirstFailure returns a short failure label from gotest output
func extractFirstFailure(output string) string {
	if strings.Contains(output, "vet ❌") {
		return "vet"
	}
	if strings.Contains(output, "timeout:") {
		return "timeout"
	}
	if strings.Contains(output, "❌") {
		return "tests"
	}
	return "failed"
}

// listCmdDirs returns the names of the subdirectories in cmd/.
// It returns an empty slice (no error) if cmd/ does not exist or is empty.
func (g *Go) listCmdDirs(rootDir string) ([]string, error) {
	cmdDir := filepath.Join(rootDir, "cmd")
	if _, err := os.Stat(cmdDir); os.IsNotExist(err) {
		return nil, nil // No cmd directory, skip silently
	}

	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cmd directory: %w", err)
	}

	var commands []string
	for _, entry := range entries {
		if entry.IsDir() {
			commands = append(commands, entry.Name())
		}
	}

	return commands, nil
}

// Install builds and installs all commands in the cmd/ directory
// It injects the version using ldflags if provided
func (g *Go) Install(version string) error {
	commands, err := g.listCmdDirs(g.rootDir)
	if err != nil {
		return err
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
		args := []string{"install"}
		if ldflags != "" {
			args = append(args, ldflags)
		}

		// If the cmd subdir has its own go.mod it is a separate module;
		// install from that directory instead of using ./cmd/<name> from root.
		cmdDir := filepath.Join(g.rootDir, "cmd", cmd)
		installDir := g.rootDir
		pkg := "./cmd/" + cmd
		if _, err := os.Stat(filepath.Join(cmdDir, "go.mod")); err == nil {
			installDir = cmdDir
			pkg = "."
		}
		args = append(args, pkg)

		if _, err := command.RunInDir(installDir, "go", args...); err != nil {
			return fmt.Errorf("failed to install %s: %w", cmd, err)
		}
	}

	g.consoleOutput(fmt.Sprintf("✅ Installed: %s", strings.Join(commands, ", ")))
	return nil
}
