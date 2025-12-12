package gitgo

import (
	"fmt"
	"os/exec"
	"strings"
)

// RunCommand executes a shell command
// It logs the command, runs it, and returns the output (trimmed)
// If the command fails, it returns an error with the output included
func RunCommand(name string, args ...string) (string, error) {
	// Log command
	cmdStr := name + " " + strings.Join(args, " ")
	log(cmdStr)

	// Execute
	cmd := exec.Command(name, args...)
	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))

	if err != nil {
		return output, fmt.Errorf("command failed: %s\nError: %w\nOutput: %s", cmdStr, err, output)
	}

	return output, nil
}

// RunCommandSilent executes a command without logging it (unless it fails)
// Useful for internal checks like "git diff-index"
func RunCommandSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))

	if err != nil {
		return output, fmt.Errorf("command failed: %s %s\nError: %w", name, strings.Join(args, " "), err)
	}

	return output, nil
}
