package devflow

import (
	"errors"
	"fmt"
	"strings"
)

// DepBump represents a single module dependency update
type DepBump struct {
	ModulePath string
	OldVersion string
	NewVersion string
}

const (
	// DepsCommitPrefix is the prefix for dependency update commit messages
	DepsCommitPrefix = "deps: "
	// CauseLinePrefix is the prefix for the root cause line in commit messages
	CauseLinePrefix = "cause: "
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

// BuildDepsCommitMessage constructs a standard dependency update commit message.
// rootCause is the original commit message that triggered the cascade.
func BuildDepsCommitMessage(bumps []DepBump, rootCause string) string {
	if len(bumps) == 0 {
		return ""
	}

	var sb strings.Builder
	// Title: deps: update module to version (if single) or multiple
	if len(bumps) == 1 {
		sb.WriteString(fmt.Sprintf("%supdate %s to %s", DepsCommitPrefix, filepathBase(bumps[0].ModulePath), bumps[0].NewVersion))
	} else {
		var names []string
		for _, b := range bumps {
			names = append(names, filepathBase(b.ModulePath))
		}
		sb.WriteString(fmt.Sprintf("%supdate %s", DepsCommitPrefix, strings.Join(names, ", ")))
	}

	if rootCause != "" {
		sb.WriteString("\n\n")
		sb.WriteString(CauseLinePrefix)
		sb.WriteString(FormatCommitMessage(rootCause))
	}

	sb.WriteString("\n")
	for _, b := range bumps {
		sb.WriteString(fmt.Sprintf("\n- %s", b.ModulePath))
		if b.OldVersion != "" {
			sb.WriteString(fmt.Sprintf(" %s →", b.OldVersion))
		}
		sb.WriteString(fmt.Sprintf(" %s", b.NewVersion))
	}

	return sb.String()
}

// filepathBase is a internal helper to avoid dependency on path/filepath in this file
// if we want to keep it light, but we already use strings.
func filepathBase(path string) string {
	if path == "" {
		return ""
	}
	i := strings.LastIndex(path, "/")
	if i >= 0 {
		return path[i+1:]
	}
	return path
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
