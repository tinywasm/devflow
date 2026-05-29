package devflow_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

// mockGoTest replaces GoTestCmdFn so ReleaseOnly does not spawn a real go test subprocess.
// Returns a function that restores the original.
func mockGoTest(t *testing.T) func() {
	t.Helper()
	orig := devflow.GoTestCmdFn
	devflow.GoTestCmdFn = func(_ context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.Command("echo", "ok  testmodule\t0.1s\ncoverage: 100% of statements in ./...")
	}
	return func() { devflow.GoTestCmdFn = orig }
}

func TestReleaseOnly_SingleCmd(t *testing.T) {
	dir, cleanup := testCreateCmdDirs(t, "goflare")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{latestTag: "v0.3.0"}
	goHandler, _ := devflow.NewGo(mockGit)
	goHandler.SetCrossCompileFn(func(tmpDir string, cmds []string, _ []devflow.CrossTarget, _ string) ([]string, error) {
		var assets []string
		for _, cmd := range cmds {
			p := filepath.Join(tmpDir, cmd+"-linux-amd64")
			os.WriteFile(p, []byte{}, 0644)
			assets = append(assets, p)
		}
		return assets, nil
	})

	fake := &fakeRunner{output: "https://github.com/org/repo/releases/tag/v0.3.0"}
	gh := newTestGitHub(fake)

	err := goHandler.ReleaseOnly("", gh)
	if err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	if fake.lastArgs[0] != "release" || fake.lastArgs[1] != "create" || fake.lastArgs[2] != "v0.3.0" {
		t.Errorf("Unexpected gh args: %v", fake.lastArgs)
	}

	assetFound := false
	for _, arg := range fake.lastArgs {
		if strings.Contains(arg, "goflare-linux-amd64") {
			assetFound = true
			break
		}
	}
	if !assetFound {
		t.Errorf("Asset not found in gh args")
	}
}

func TestReleaseOnly_MultipleCmd(t *testing.T) {
	dir, cleanup := testCreateCmdDirs(t, "gopush", "gotest")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{latestTag: "v0.1.0"}
	goHandler, _ := devflow.NewGo(mockGit)
	goHandler.SetCrossCompileFn(func(tmpDir string, cmds []string, targets []devflow.CrossTarget, _ string) ([]string, error) {
		var assets []string
		for _, target := range targets {
			for _, cmd := range cmds {
				suffix := "-" + target.GOOS + "-" + target.GOARCH
				if target.GOOS == "windows" {
					suffix += ".exe"
				}
				p := filepath.Join(tmpDir, cmd+suffix)
				os.WriteFile(p, []byte{}, 0644)
				assets = append(assets, p)
			}
		}
		return assets, nil
	})

	fake := &fakeRunner{output: "https://github.com/org/repo/releases/tag/v0.1.0"}
	gh := newTestGitHub(fake)

	err := goHandler.ReleaseOnly("", gh)
	if err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	// 2 cmds * 3 platforms = 6 assets
	assetCount := 0
	for _, arg := range fake.lastArgs {
		if strings.Contains(arg, "gopush-") || strings.Contains(arg, "gotest-") {
			assetCount++
		}
	}
	if assetCount != 6 {
		t.Errorf("Expected 6 assets, got %d", assetCount)
	}
}

func TestReleaseOnly_ExplicitTag(t *testing.T) {
	dir, cleanup := testCreateCmdDirs(t, "mytool")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{latestTag: "v0.5.0"}
	goHandler, _ := devflow.NewGo(mockGit)
	goHandler.SetCrossCompileFn(func(tmpDir string, cmds []string, _ []devflow.CrossTarget, _ string) ([]string, error) {
		return []string{filepath.Join(tmpDir, "mytool-linux-amd64")}, nil
	})

	fake := &fakeRunner{output: "https://github.com/org/repo/releases/tag/v1.0.0"}
	gh := newTestGitHub(fake)

	// Explicit tag v1.0.0 should be used, not GetLatestTag() result
	err := goHandler.ReleaseOnly("v1.0.0", gh)
	if err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	if fake.lastArgs[2] != "v1.0.0" {
		t.Errorf("Expected tag v1.0.0, got %s", fake.lastArgs[2])
	}
}

func TestReleaseOnly_UsesLatestTag(t *testing.T) {
	dir, cleanup := testCreateCmdDirs(t, "mytool")
	defer cleanup()
	defer testChdir(t, dir)()

	mockGit := &MockGitClient{latestTag: "v2.5.3"}
	goHandler, _ := devflow.NewGo(mockGit)
	goHandler.SetCrossCompileFn(func(tmpDir string, cmds []string, _ []devflow.CrossTarget, _ string) ([]string, error) {
		return []string{filepath.Join(tmpDir, "mytool-linux-amd64")}, nil
	})

	fake := &fakeRunner{output: "https://github.com/org/repo/releases/tag/v2.5.3"}
	gh := newTestGitHub(fake)

	// No explicit tag -> should call GetLatestTag()
	err := goHandler.ReleaseOnly("", gh)
	if err != nil {
		t.Fatalf("ReleaseOnly failed: %v", err)
	}

	if fake.lastArgs[2] != "v2.5.3" {
		t.Errorf("Expected tag v2.5.3 from GetLatestTag, got %s", fake.lastArgs[2])
	}
}

func TestCrossCompile_NamingConvention(t *testing.T) {
	repoDir, cleanup := testCreateCmdDirs(t, "mytool")
	defer cleanup()

	tmpDir, err := os.MkdirTemp("", "gorelease-naming-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	assets, err := devflow.CrossCompile(tmpDir, []string{"mytool"}, devflow.DefaultTargets(), repoDir)
	if err != nil {
		t.Fatalf("CrossCompile failed: %v", err)
	}

	expectedSuffixes := []string{"mytool-linux-amd64", "mytool-darwin-arm64", "mytool-windows-amd64.exe"}
	for _, suffix := range expectedSuffixes {
		found := false
		for _, asset := range assets {
			if strings.HasSuffix(asset, suffix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected asset suffix %s not found", suffix)
		}
	}
}

func TestCreateRelease_Args(t *testing.T) {
	fake := &fakeRunner{output: "https://github.com/org/repo/releases/tag/v1.0.0"}
	gh := newTestGitHub(fake)

	url, err := gh.CreateRelease("v1.0.0", []string{"/tmp/a", "/tmp/b"})
	if err != nil {
		t.Fatalf("CreateRelease failed: %v", err)
	}

	expected := []string{"release", "create", "v1.0.0", "--title", "v1.0.0", "--notes", "", "/tmp/a", "/tmp/b"}
	for i, arg := range expected {
		if fake.lastArgs[i] != arg {
			t.Errorf("Expected arg %d to be %s, got %s", i, arg, fake.lastArgs[i])
		}
	}
	if url != "https://github.com/org/repo/releases/tag/v1.0.0" {
		t.Errorf("Unexpected URL: %s", url)
	}
}

func TestReleaseOnly_Errors(t *testing.T) {
	t.Run("No cmd dir", func(t *testing.T) {
		dir, cleanup := testCreateCmdDirs(t) // No args = no cmd dir
		defer cleanup()
		defer testChdir(t, dir)()

		goHandler, _ := devflow.NewGo(&MockGitClient{})
		err := goHandler.ReleaseOnly("v1.0.0", &devflow.GitHub{})
		if err == nil || !strings.Contains(err.Error(), "no cmd/ found") {
			t.Errorf("Expected 'no cmd/ found' error, got %v", err)
		}
	})

	t.Run("No tags when tag empty", func(t *testing.T) {
		dir, cleanup := testCreateCmdDirs(t, "tool")
		defer cleanup()
		defer testChdir(t, dir)()

		mockGit := &MockGitClient{latestTag: ""} // No tags
		goHandler, _ := devflow.NewGo(mockGit)
		err := goHandler.ReleaseOnly("", &devflow.GitHub{})
		if err == nil || !strings.Contains(err.Error(), "no tags found") {
			t.Errorf("Expected 'no tags found' error, got %v", err)
		}
	})

	t.Run("TmpDir cleaned up on cross-compile error", func(t *testing.T) {
		dir, cleanup := testCreateCmdDirs(t, "tool")
		defer cleanup()
		defer testChdir(t, dir)()

		mockGit := &MockGitClient{latestTag: "v1"}
		goHandler, _ := devflow.NewGo(mockGit)

		var capturedTmpDir string
		goHandler.SetCrossCompileFn(func(tmpDir string, cmds []string, _ []devflow.CrossTarget, _ string) ([]string, error) {
			capturedTmpDir = tmpDir
			return nil, errors.New("compile error")
		})

		err := goHandler.ReleaseOnly("", &devflow.GitHub{})
		if err == nil {
			t.Fatal("Expected error")
		}

		if _, err := os.Stat(capturedTmpDir); !os.IsNotExist(err) {
			t.Errorf("TmpDir %s was not cleaned up", capturedTmpDir)
		}
	})
}

func TestCodeJob_ReleaseFlag(t *testing.T) {
	// Test that SetReleaser is called when -release flag is used in close-loop
	mockPublisher := &MockPublisher{
		PublishFn: func(message, tag string, skipTests, skipRace, skipDependents, skipBackup, skipTag, skipVerify bool) (devflow.PushResult, error) {
			return devflow.PushResult{
				Summary: "gopush ok",
				Tag:     "v1.2.0",
			}, nil
		},
	}

	job := devflow.NewCodeJob()
	job.SetPublisher(mockPublisher)

	releaseCalled := false
	var releaseTag string
	job.SetReleaser(func(tag string) error {
		releaseCalled = true
		releaseTag = tag
		return nil
	})

	// Set up .env with a pending PR to simulate close-loop scenario
	env := devflow.NewDotEnv(".env")
	_ = env.Set(devflow.EnvKeyCodejobPR, "https://github.com/owner/repo/pull/1")

	// Run with -release flag
	res, err := job.Run("fix: bug", "", true)
	if err != nil {
		t.Fatalf("Run with -release failed: %v", err)
	}

	if !releaseCalled {
		t.Errorf("Release function should have been called with -release flag")
	}
	if releaseTag != "v1.2.0" {
		t.Errorf("Expected release tag v1.2.0, got %s", releaseTag)
	}

	if !strings.Contains(res, "gopush ok") {
		t.Errorf("Expected gopush result in response, got: %s", res)
	}
}

func TestCodeJob_NoReleaseFlag(t *testing.T) {
	// Test that SetReleaser is NOT called when -release flag is absent in close-loop
	mockPublisher := &MockPublisher{
		PublishFn: func(message, tag string, skipTests, skipRace, skipDependents, skipBackup, skipTag, skipVerify bool) (devflow.PushResult, error) {
			return devflow.PushResult{
				Summary: "gopush ok",
				Tag:     "v1.2.0",
			}, nil
		},
	}

	job := devflow.NewCodeJob()
	job.SetPublisher(mockPublisher)

	releaseCalled := false
	job.SetReleaser(func(tag string) error {
		releaseCalled = true
		return nil
	})

	// Set up .env with a pending PR to simulate close-loop scenario
	env := devflow.NewDotEnv(".env")
	_ = env.Set(devflow.EnvKeyCodejobPR, "https://github.com/owner/repo/pull/1")

	// Run WITHOUT -release flag
	_, err := job.Run("fix: bug", "", false)
	if err != nil {
		t.Fatalf("Run without -release failed: %v", err)
	}

	if releaseCalled {
		t.Errorf("Release function should not have been called without -release flag")
	}
}
