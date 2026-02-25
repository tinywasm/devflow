package devflow

import (
	"os"
	"strings"
)

// DotEnv handles .env files while trying to preserve non-key=value lines (comments, empty lines).
type DotEnv struct {
	path string
}

// NewDotEnv creates a new .env handler.
func NewDotEnv(path string) *DotEnv {
	return &DotEnv{path: path}
}

// Get retrieves a value from the .env file.
func (e *DotEnv) Get(key string) (string, bool) {
	data, err := os.ReadFile(e.path)
	if err != nil {
		return "", false
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1]), true
		}
	}
	return "", false
}

// Set sets or updates a value in the .env file, preserving other lines.
func (e *DotEnv) Set(key, value string) error {
	var lines []string
	data, err := os.ReadFile(e.path)
	if err == nil {
		lines = strings.Split(string(data), "\n")
	}

	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}

	if !found {
		// If the last line is not empty and we have lines, add an empty line before append
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, key+"="+value)
	}

	return os.WriteFile(e.path, []byte(strings.Join(lines, "\n")), 0644)
}

// Delete removes a key from the .env file.
func (e *DotEnv) Delete(key string) error {
	data, err := os.ReadFile(e.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	changed := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			changed = true
			continue
		}
		newLines = append(newLines, line)
	}

	if !changed {
		return nil
	}

	return os.WriteFile(e.path, []byte(strings.Join(newLines, "\n")), 0644)
}
