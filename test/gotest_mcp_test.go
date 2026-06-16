package devflow_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/tinywasm/devflow"
)

func TestGoTestProvider(t *testing.T) {
	// Create a real temp module for the tests to pass g.Test initial checks
	dir, cleanup := testCreateGoModule("example.com/mcptest")
	defer cleanup()

	// Setup
	git, _ := devflow.NewGit()
	g, _ := devflow.NewGo(git)
	g.SetRootDir(dir)

	var logs []string
	g.SetLog(func(args ...any) {
		logs = append(logs, args[0].(string))
	})

	provider := devflow.NewGoTestProvider(g)

	// Mock GoTestCmdFn to capture arguments
	var capturedDir string
	var capturedArgs []string

	// Preserve original
	originalGoTestCmdFn := devflow.GoTestCmdFn
	defer func() { devflow.GoTestCmdFn = originalGoTestCmdFn }()

	devflow.GoTestCmdFn = func(ctx context.Context, dir string, name string, args ...string) *exec.Cmd {
		capturedDir = dir
		capturedArgs = args
		// Return a command that exits with success
		return exec.Command("true")
	}

	t.Run("Full Suite (no args)", func(t *testing.T) {
		logs = nil
		capturedDir = ""
		capturedArgs = nil

		tools := provider.GetMCPTools()
		if len(tools) != 1 || tools[0].Name != "run_tests" {
			t.Fatalf("Expected 1 tool named run_tests")
		}

		tools[0].Execute(make(map[string]any))

		if capturedDir != dir {
			t.Errorf("Expected dir %s, got %q", dir, capturedDir)
		}

		// Full suite should NOT have -run but should have -v, -cover, etc. (from runFullTestSuite)
		foundRun := false
		for _, arg := range capturedArgs {
			if arg == "-run" {
				foundRun = true
			}
		}
		if foundRun {
			t.Error("Full suite should not have -run arg")
		}

		if len(logs) == 0 {
			t.Error("Expected output in logs")
		}
	})

	t.Run("Single Test (run arg)", func(t *testing.T) {
		logs = nil
		capturedDir = ""
		capturedArgs = nil

		tools := provider.GetMCPTools()
		tools[0].Execute(map[string]any{"run": "TestFoo"})

		if capturedDir != dir {
			t.Errorf("Expected dir %s, got %q", dir, capturedDir)
		}

		foundRun := false
		for i, arg := range capturedArgs {
			if arg == "-run" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "TestFoo" {
				foundRun = true
				break
			}
		}
		if !foundRun {
			t.Errorf("Single test run missing correct -run arg. Args: %v", capturedArgs)
		}

		if len(logs) == 0 {
			t.Error("Expected output in logs")
		}
	})
}
