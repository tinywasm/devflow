package devflow

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// RunCommand executes a shell command
// It returns the output (trimmed) and an error if the command fails
func RunCommand(name string, args ...string) (string, error) {
	// Execute
	cmd := exec.Command(name, args...)
	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))

	if err != nil {
		cmdStr := name + " " + strings.Join(args, " ")
		return output, fmt.Errorf("command failed: %s\nError: %w\nOutput: %s", cmdStr, err, output)
	}

	return output, nil
}

// RunCommandSilent executes a command (alias for RunCommand now, as RunCommand is also silent on success)
// kept for backward compatibility if needed, or we can remove it.
// The previous implementation was identical except for logging.
func RunCommandSilent(name string, args ...string) (string, error) {
	return RunCommand(name, args...)
}

// RunShellCommand executes a shell command in a cross-platform way
// On Windows: uses cmd.exe /C
// On Unix (Linux/macOS): uses sh -c
func RunShellCommand(command string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		return RunCommand("cmd.exe", "/C", command)
	default: // linux, darwin, etc.
		return RunCommand("sh", "-c", command)
	}
}

// RunShellCommandAsync starts a shell command asynchronously (non-blocking)
// Returns immediately after starting, does not wait for completion
func RunShellCommandAsync(command string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd.exe", "/C", command)
	default: // linux, darwin, etc.
		cmd = exec.Command("sh", "-c", command)
	}

	return cmd.Start()
}

// RunCommandWithRetry executes a command with retries
// It waits for 'delay' duration between retries
func RunCommandWithRetry(name string, args []string, maxRetries int, delay time.Duration) (string, error) {
	var output string
	var err error

	for i := 0; i < maxRetries; i++ {
		output, err = RunCommand(name, args...)
		if err == nil {
			return output, nil
		}

		// If this was the last attempt, return the error
		if i == maxRetries-1 {
			break
		}

		// Wait before retrying
		time.Sleep(delay)
	}
	return output, fmt.Errorf("command %s failed after %d attempts: %w", name, maxRetries, err)
}

// RunCommandInDir executes a command in a specific directory
func RunCommandInDir(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))

	if err != nil {
		cmdStr := name + " " + strings.Join(args, " ")
		return output, fmt.Errorf("command failed in %s: %s\nError: %w\nOutput: %s", dir, cmdStr, err, output)
	}

	return output, nil
}

// RunCommandWithRetryInDir executes a command in a specific directory with retries
func RunCommandWithRetryInDir(dir, name string, args []string, maxRetries int, delay time.Duration) (string, error) {
	var output string
	var err error

	for i := 0; i < maxRetries; i++ {
		output, err = RunCommandInDir(dir, name, args...)
		if err == nil {
			return output, nil
		}

		if i == maxRetries-1 {
			break
		}
		time.Sleep(delay)
	}

	return output, fmt.Errorf("command %s failed in %s after %d attempts: %w", name, dir, maxRetries, err)
}
