package devflow_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

// scriptedRunner implements devflow.SecretRunner and returns different outputs
// depending on the command issued. It records every call so tests can inspect
// the final `gh release create` invocation.
//
// respond receives the args (without the leading binary name) and returns the
// simulated stdout + error.
type scriptedRunner struct {
	calls   [][]string
	respond func(args []string) (string, error)
}

func (s *scriptedRunner) record(args []string) {
	cp := make([]string, len(args))
	copy(cp, args)
	s.calls = append(s.calls, cp)
}

func (s *scriptedRunner) Run(name string, args ...string) (string, error) {
	s.record(args)
	return s.respond(args)
}

func (s *scriptedRunner) RunSilent(name string, args ...string) (string, error) {
	s.record(args)
	return s.respond(args)
}

func (s *scriptedRunner) RunWithStdin(input, name string, args ...string) (string, error) {
	s.record(args)
	return s.respond(args)
}

// lastReleaseCreateArgs returns the args of the last `gh release create` call.
func (s *scriptedRunner) lastReleaseCreateArgs() []string {
	for i := len(s.calls) - 1; i >= 0; i-- {
		c := s.calls[i]
		if len(c) >= 2 && c[0] == "release" && c[1] == "create" {
			return c
		}
	}
	return nil
}

// countReleaseCreate returns how many `gh release create` calls were made.
func (s *scriptedRunner) countReleaseCreate() int {
	n := 0
	for _, c := range s.calls {
		if len(c) >= 2 && c[0] == "release" && c[1] == "create" {
			n++
		}
	}
	return n
}

// isRepoView reports whether args correspond to a `gh repo view` call.
func isRepoView(args []string) bool {
	return len(args) >= 2 && args[0] == "repo" && args[1] == "view"
}

// repoViewTarget returns the positional repo ref of a `gh repo view` call,
// or "" when the call targets the current directory (no positional ref).
func repoViewTarget(args []string) string {
	if len(args) >= 3 && args[2] != "--json" {
		return args[2]
	}
	return ""
}

// newGitHubWithRunner builds a *GitHub with an injected SecretRunner.
func newGitHubWithRunner(r devflow.SecretRunner) *devflow.GitHub {
	gh := &devflow.GitHub{}
	gh.SecretRunner = r
	return gh
}

// createAppDir creates a temp Go module whose folder basename is exactly
// folderName (e.g. "app"), with a cmd/<cmd> subdir, and chdirs into it.
func createAppDir(t *testing.T, folderName, cmd string) func() {
	t.Helper()
	base, err := os.MkdirTemp("", "release-pub-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	appDir := filepath.Join(base, folderName)
	cmdDir := filepath.Join(appDir, "cmd", cmd)
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("mkdir cmd: %v", err)
	}
	os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module testmodule\n\ngo 1.20\n"), 0644)
	os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	old, _ := os.Getwd()
	if err := os.Chdir(appDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return func() {
		os.Chdir(old)
		os.RemoveAll(base)
	}
}

// fakeCrossCompile produces one empty asset per cmd in tmpDir.
func fakeCrossCompile(tmpDir string, cmds []string, _ []devflow.CrossTarget, _ string) ([]string, error) {
	var assets []string
	for _, cmd := range cmds {
		p := filepath.Join(tmpDir, cmd+"-linux-amd64")
		os.WriteFile(p, []byte{}, 0644)
		assets = append(assets, p)
	}
	return assets, nil
}

func argsContainRepo(args []string, repo string) bool {
	for i, a := range args {
		if a == "--repo" && i+1 < len(args) && args[i+1] == repo {
			return true
		}
	}
	return false
}

func argsHaveRepoFlag(args []string) bool {
	for _, a := range args {
		if a == "--repo" {
			return true
		}
	}
	return false
}

// EXPECTED FLOW 1: origin is private → gorelease derives <owner>/<folder> and,
// after verifying it is public, publishes the release there with --repo.
func TestReleaseOnly_PrivateOrigin_PublishesToDerivedPublicRepo(t *testing.T) {
	cleanup := createAppDir(t, "app", "tinywasm")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			switch repoViewTarget(args) {
			case "": // origin (current dir) → PRIVATE core
				return `{"owner":{"login":"tinywasm"},"name":"core","visibility":"PRIVATE"}`, nil
			case "tinywasm/app": // derived candidate → PUBLIC
				return `{"owner":{"login":"tinywasm"},"name":"app","visibility":"PUBLIC"}`, nil
			}
		}
		// release create
		return "https://github.com/tinywasm/app/releases/tag/v0.3.0", nil
	}}

	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v0.3.0"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	if err := goHandler.ReleaseOnly("", gh); err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	args := runner.lastReleaseCreateArgs()
	if args == nil {
		t.Fatal("no `gh release create` call recorded")
	}
	if !argsContainRepo(args, "tinywasm/app") {
		t.Errorf("expected release published to tinywasm/app via --repo, got args: %v", args)
	}
}

// EXPECTED FLOW 2: origin is public → classic behavior, publish to origin,
// no --repo flag.
func TestReleaseOnly_PublicOrigin_PublishesToOrigin(t *testing.T) {
	cleanup := createAppDir(t, "mytool", "mytool")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			return `{"owner":{"login":"acme"},"name":"mytool","visibility":"PUBLIC"}`, nil
		}
		return "https://github.com/acme/mytool/releases/tag/v1.0.0", nil
	}}

	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v1.0.0"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	if err := goHandler.ReleaseOnly("", gh); err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	args := runner.lastReleaseCreateArgs()
	if args == nil {
		t.Fatal("no `gh release create` call recorded")
	}
	if argsHaveRepoFlag(args) {
		t.Errorf("public origin must publish to origin without --repo, got args: %v", args)
	}
}

// EXPECTED FLOW 3: origin private but the derived public repo does not exist /
// is not public → fail loudly, do not publish a useless private release.
func TestReleaseOnly_PrivateOrigin_DerivedRepoNotPublic_Errors(t *testing.T) {
	cleanup := createAppDir(t, "app", "tinywasm")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			switch repoViewTarget(args) {
			case "": // origin → PRIVATE
				return `{"owner":{"login":"tinywasm"},"name":"core","visibility":"PRIVATE"}`, nil
			case "tinywasm/app": // candidate is also PRIVATE (or missing)
				return `{"owner":{"login":"tinywasm"},"name":"app","visibility":"PRIVATE"}`, nil
			}
		}
		return "should-not-reach-release-create", nil
	}}

	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v0.3.0"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	err := goHandler.ReleaseOnly("", gh)
	if err == nil {
		t.Fatal("expected error when derived repo is not public, got nil")
	}
	if runner.lastReleaseCreateArgs() != nil {
		t.Errorf("must not call `gh release create` when no public target exists")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "public") {
		t.Errorf("error should explain there is no public repo to publish to, got: %v", err)
	}
}

// EXPECTED FLOW 4: visibility cannot be determined (gh repo view fails / non-JSON)
// → backward-compatible fallback to origin (no --repo, release still happens).
func TestReleaseOnly_VisibilityUndetermined_FallsBackToOrigin(t *testing.T) {
	cleanup := createAppDir(t, "app", "tinywasm")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			// Non-JSON garbage → resolution cannot determine visibility.
			return "not json", nil
		}
		return "https://github.com/tinywasm/core/releases/tag/v0.3.0", nil
	}}

	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v0.3.0"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	if err := goHandler.ReleaseOnly("", gh); err != nil {
		t.Fatalf("fallback to origin must not fail: %v", err)
	}

	args := runner.lastReleaseCreateArgs()
	if args == nil {
		t.Fatal("no `gh release create` call recorded")
	}
	if argsHaveRepoFlag(args) {
		t.Errorf("undetermined visibility must fall back to origin without --repo, got args: %v", args)
	}
}

// EXPECTED FLOW 3b (diagram "¿existe y es público?" → No existe): origin private
// but the derived repo does not exist at all (`gh repo view <candidate>` errors).
// Same outcome as "exists but private": fail loudly, do not publish.
func TestReleaseOnly_PrivateOrigin_DerivedRepoMissing_Errors(t *testing.T) {
	cleanup := createAppDir(t, "app", "tinywasm")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			switch repoViewTarget(args) {
			case "": // origin → PRIVATE
				return `{"owner":{"login":"tinywasm"},"name":"core","visibility":"PRIVATE"}`, nil
			case "tinywasm/app": // candidate does not exist
				return "", errors.New("GraphQL: Could not resolve to a Repository with the name 'tinywasm/app'")
			}
		}
		return "should-not-reach-release-create", nil
	}}

	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v0.3.0"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	err := goHandler.ReleaseOnly("", gh)
	if err == nil {
		t.Fatal("expected error when derived repo does not exist, got nil")
	}
	if runner.lastReleaseCreateArgs() != nil {
		t.Errorf("must not call `gh release create` when derived repo is missing")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "public") {
		t.Errorf("error should explain there is no public repo to publish to, got: %v", err)
	}
}

// EXPECTED FLOW (diagram "Release ok? No → Exit 1 gh error"): once a target is
// resolved, a failing `gh release create` must propagate the error.
func TestReleaseOnly_ReleaseCreateFails_PropagatesError(t *testing.T) {
	cleanup := createAppDir(t, "mytool", "mytool")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			return `{"owner":{"login":"acme"},"name":"mytool","visibility":"PUBLIC"}`, nil
		}
		// release create fails
		return "", errors.New("HTTP 403: release already exists")
	}}

	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v1.0.0"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	err := goHandler.ReleaseOnly("", gh)
	if err == nil {
		t.Fatal("expected error when `gh release create` fails, got nil")
	}
	if runner.lastReleaseCreateArgs() == nil {
		t.Error("release create should have been attempted")
	}
}

// EXPECTED (mejora #1, seguridad): gorelease must publish a checksums file
// (SHA256 of every binary) alongside the assets, so the installer can verify
// integrity before making a downloaded binary executable.
func TestReleaseOnly_UploadsChecksums(t *testing.T) {
	cleanup := createAppDir(t, "mytool", "mytool")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			return `{"owner":{"login":"acme"},"name":"mytool","visibility":"PUBLIC"}`, nil
		}
		return "https://github.com/acme/mytool/releases/tag/v1.0.0", nil
	}}

	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v1.0.0"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	if err := goHandler.ReleaseOnly("", gh); err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	args := runner.lastReleaseCreateArgs()
	checksumFound := false
	for _, a := range args {
		if strings.HasSuffix(a, "checksums.txt") {
			checksumFound = true
			break
		}
	}
	if !checksumFound {
		t.Errorf("expected a checksums.txt asset in release create, got args: %v", args)
	}
}

// EXPECTED (mejora #4): gorelease publishes EXACTLY ONE release for the highest
// semver tag (resolved by git.GetLatestTag), never one release per tag.
// Regression lock — this is already the behavior; keep it that way.
func TestReleaseOnly_PublishesSingleReleaseForHighestTag(t *testing.T) {
	cleanup := createAppDir(t, "mytool", "mytool")
	defer cleanup()

	runner := &scriptedRunner{respond: func(args []string) (string, error) {
		if isRepoView(args) {
			return `{"owner":{"login":"acme"},"name":"mytool","visibility":"PUBLIC"}`, nil
		}
		return "https://github.com/acme/mytool/releases/tag/v2.5.3", nil
	}}

	// MockGitClient.GetLatestTag returns the highest semver tag.
	goHandler, _ := devflow.NewGo(&MockGitClient{latestTag: "v2.5.3"})
	goHandler.SetCrossCompileFn(fakeCrossCompile)
	gh := newGitHubWithRunner(runner)

	if err := goHandler.ReleaseOnly("", gh); err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	if n := runner.countReleaseCreate(); n != 1 {
		t.Errorf("expected exactly 1 release create for the highest tag, got %d", n)
	}
	if args := runner.lastReleaseCreateArgs(); len(args) < 3 || args[2] != "v2.5.3" {
		t.Errorf("release must target the highest tag v2.5.3, got args: %v", args)
	}
}
