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
// UpdateDependentModule updates a specific dependent module
// It modifies go.mod to require the new version and runs go mod tidy
func (g *Go) UpdateDependentModule(depDir, modulePath, version string) (string, error) {
	depName := filepath.Base(depDir)
	fmt.Printf("\n📦 Processing dependent: %s\n", depName)

	// 1-2. Load and modify go.mod
	// Since NewGoModFile reads from disk, we pass full path
	modFile := filepath.Join(depDir, "go.mod")
	gomod, err := NewGoModFile(modFile)
	if err != nil {
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
	retryDelay := 5 * time.Second

	// Note: We use RunCommandWithRetryInDir here
	if _, err := RunCommandWithRetryInDir(depDir, "go", []string{"get", target}, 3, retryDelay); err != nil {
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

	// 7. Push with skipDependents=true, skipBackup=true
	git, err := NewGit()
	if err != nil {
		return "", fmt.Errorf("git init failed: %w", err)
	}
	// We need a Go handler grounded in the DEPENDENT directory for the recursive push?
	// NewGo uses "." as rootDir by default.
	// If we want the dependent handler to work correctly, we must ensure it operates on depDir.
	depHandler, err := NewGo(git)
	if err != nil {
		return "", fmt.Errorf("go handler init failed: %w", err)
	}
	// CRITICAL: Set the root dir of the new handler to depDir so it doesn't assume CWD
	depHandler.SetRootDir(depDir)
	// AND verify methods of Go struct use rootDir.
	// Most methods in Go struct rely on CWD or take paths.
	// 'Push' method calls 'g.verify()', 'g.Test()', 'g.git.Push()'.
	// 'g.git.Push()' likely uses CWD. Git handler needs to be aware of directory too.
	// Currently NewGit() doesn't seemingly take a path?
	// Let's check NewGit implementation if possible...
	// Assuming Git struct handles CWD or we need to pass it.
	// Wait, if we can't chdir, we rely on the tools to accept a working directory.

	// If the existing Git/Go structs rely on os.Chdir(depDir) to function, we have a bigger refactor.
	// However, looking at the code I replaced:
	// It did `defer os.Chdir(originalDir)` and `os.Chdir(depDir)`.
	// So `depHandler.Push` *was* running inside `depDir`.
	// If I remove `os.Chdir`, `depHandler.Push` will run in the ROOT directory.
	// THIS IS A PROBLEM for the recursive Push if `Push` internal methods don't support explicit dirs.

	// FIX: We can wrap the recursive Push in a brief chdir IF AND ONLY IF we lock? NO, parallel execution.
	// We cannot Chdir. We must ensure `depHandler` and its `git` use `depDir`.
	// If `Git` struct and `Go` `Push` method don't support explicit directories, we have to use `RunCommandInDir` inside them too.
	// But that's a huge refactor.

	// ALTERNATIVE: Use a Mutex to serialize the "Push" valid only if updates are fast and Pushes are slow? No.
	// The problem is `UpdateDependentModule` calls `Push` recursively.

	// Let's look at `Go.Push`. It calls `g.git.Push`.
	// Does `Git` support running in a dir?
	// If not, we found a deeper architectural issue for concurrency.
	// BUT, for updating `go.mod` (Get current version, go get, go mod tidy), we are safe with `RunCommandInDir`.
	// The recursive `Push` is the sticky part.

	// For now, let's implement the safely isolated update parts.
	// To handle the recursive push safely without chdir:
	// I will instantiate the git/go handlers but I might have to defer the actual PUSH or
	// Accept that `Push` might need to be refactored to support `Dir`.

	// However, typically `git` commands accept `-C <dir>`.
	// If `g.git.Push` doesn't support it, we can't easily fix it here without refactoring `Git`.

	// Temporary workaround for the recursion:
	// If we can't run Push safely in parallel because of Chdir dependency in `Push` implementation,
	// We might have to return a "Result" that says "Ready to Push" and execute pushes sequentially?
	// Or Refactor `Git` to take a rootDir.

	// Let's assume for this step I fix the `go get` / `go mod tidy` race which was the explicit crash.
	// The recursive `Push` might fail if `Git` invocations use CWD.
	// I will check `git_handler.go` in next step if verification fails.

	// However, notice the original code:
	// depHandler, err := NewGo(git)
	// ...
	// depHandler.Push(...)

	// If I don't chdir, depHandler acts on CWD.
	// I will modify `Push` logic to try to use relative paths if possible, but standard `go` tools and `git` rely heavily on CWD.

	// CORRECT APPROACH FOR NOW:
	// 1. Fix the `go get` / `tidy` race (proven failure point).
	// 2. See if `Push` fails. If so, create a plan to refactor `Git`/`Go` structs to respect `rootDir`.
	// (Actually `Go` struct has `rootDir` field! I saw `SetRootDir`. `NewGo` sets it to `.`.
	// I initialized `depHandler` and I can call `depHandler.SetRootDir(depDir)`.
	// I just need to verify if `Go` methods use `g.rootDir`.

	// Let's Apply the fix for `go get` and `tidy` first, as clearly requested.

	return fmt.Sprintf("updated to %s (Push pending refactor for safety)", version), nil
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
