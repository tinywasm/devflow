package devflow

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// GitIgnoreAdd adds entry to .gitignore if shouldWrite allows and entry not present.
// Creates .gitignore if it doesn't exist.
func (g *Git) GitIgnoreAdd(entry string) error {
	if g.shouldWrite != nil && !g.shouldWrite() {
		return nil
	}

	// Check if already contains
	contains, err := g.gitIgnoreContains(entry)
	if err != nil {
		return err
	}
	if contains {
		return nil
	}

	// Append to file (create if not exists)
	path := filepath.Join(g.rootDir, ".gitignore")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(entry + "\n")
	return err
}

// gitIgnoreContains checks if an entry exists in .gitignore.
func (g *Git) gitIgnoreContains(entry string) (bool, error) {
	path := filepath.Join(g.rootDir, ".gitignore")

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == entry {
			return true, nil
		}
	}

	return false, scanner.Err()
}
