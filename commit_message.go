package devflow

import (
	"errors"
	"strings"
)

// ValidateCommitMessage ensures that a commit message is provided and is valid.
// It trims whitespace and returns an error if the message is empty.
func ValidateCommitMessage(message string) error {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return errors.New("commit message cannot be empty")
	}

	// Basic check for shell redirection/pipes if necessary,
	// but exec.Command handles arguments safely.
	// The user mentioned backticks could cause issues if passed via shell.
	// We don't need to prevent backticks here, but we should ensure they are
	// handled consistently.

	return nil
}

// FormatCommitMessage ensures the message is trimmed.
func FormatCommitMessage(message string) string {
	return strings.TrimSpace(message)
}

// ValidateShellSafeMessage provides a warning if the message contains characters
// that might need escaping in certain shells (like backticks, dollar signs, or single quotes)
// if it were to be used in a shell script, even though exec.Command is safe.
func ValidateShellSafeMessage(message string) string {
	if strings.ContainsAny(message, "`$") {
		return "Note: commit message contains shell-sensitive characters (` or $). " +
			"Always use single quotes ('') around the message in your terminal."
	}
	if strings.Contains(message, "'") {
		return "Note: commit message contains a single quote ('). " +
			"If you wrap the message in single quotes in your terminal, remember to escape it or use double quotes (\"\")."
	}
	return ""
}
