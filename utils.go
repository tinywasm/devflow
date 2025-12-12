package gitgo

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// utils.go contains shared utilities for handlers

// RunOptions configures command execution
type RunOptions struct {
	Dir string
}

// commandRunner is a function type for running commands
// This allows for mocking in tests
type commandRunner func(opts *RunOptions, name string, args ...string) (string, error)

var (
	// defaultRunner is the default implementation using exec.Command
	defaultRunner commandRunner = func(opts *RunOptions, name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
        if opts != nil && opts.Dir != "" {
            cmd.Dir = opts.Dir
        }
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}

	// currentRunner is the active runner
	currentRunner = defaultRunner
)

// runCommand executes a command and returns output + error
// It respects silent/verbose mode via logger
func runCommand(name string, args ...string) (string, error) {
    return runCommandWithOpts(nil, name, args...)
}

func runCommandWithOpts(opts *RunOptions, name string, args ...string) (string, error) {
	cmdStr := name + " " + strings.Join(args, " ")
    if opts != nil && opts.Dir != "" {
        cmdStr += " (in " + opts.Dir + ")"
    }
	log(cmdStr)

	out, err := currentRunner(opts, name, args...)
	if err != nil {
		// Do not log error here to avoid double logging (caller handles it)
		return out, fmt.Errorf("command '%s' failed: %w (output: %s)", cmdStr, err, out)
	}
	return out, nil
}

// runCommandSilent executes a command without logging the command string
func runCommandSilent(name string, args ...string) (string, error) {
	out, err := currentRunner(nil, name, args...)
	if err != nil {
		return out, fmt.Errorf("command '%s %s' failed: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}

// Helper for tests to mock the runner
func mockRunner(t *testing.T, mock func(name string, args ...string) (string, error)) {
    // Adapter for old style mock signature
	old := currentRunner
	currentRunner = func(opts *RunOptions, name string, args ...string) (string, error) {
        return mock(name, args...)
    }
	t.Cleanup(func() {
		currentRunner = old
	})
}

// Helper for tests to mock the runner with options support
func mockRunnerWithOpts(t *testing.T, mock func(opts *RunOptions, name string, args ...string) (string, error)) {
	old := currentRunner
	currentRunner = mock
	t.Cleanup(func() {
		currentRunner = old
	})
}
