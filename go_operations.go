package gitgo

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// GoModVerify verifies go.mod integrity
func GoModVerify() error {
	if !goModExists() {
		return fmt.Errorf("go.mod not found")
	}

	_, err := RunCommand("go", "mod", "verify")
	return err
}

// GoModTidy runs go mod tidy
func GoModTidy() error {
	_, err := RunCommand("go", "mod", "tidy")
	return err
}

// GoTest runs tests
func GoTest() error {
	log("Running tests...")
	// CombinedOutput is handled by RunCommand, but we might want to see output even on success?
	// RunCommand logs the command. If it fails, it returns error with output.
	// If we want to see "PASS" output, we might need to print the result.
	_, err := RunCommand("go", "test", "./...")
	if err != nil {
		return err
	}
	// Optional: log output if verbose
	return nil
}

// GoTestRace runs tests with race detector
func GoTestRace() error {
	log("Running race detector...")
	_, err := RunCommand("go", "test", "-race", "./...")
	return err
}

// GoGetModuleName gets module name from go.mod
// Extracts: module github.com/user/repo -> repo
func GoGetModuleName() (string, error) {
	modPath, err := GoGetModulePath()
	if err != nil {
		return "", err
	}

	// Extract last part of path
	parts := strings.Split(modPath, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid module path: %s", modPath)
	}

	return parts[len(parts)-1], nil
}

// GoGetModulePath gets full module path
// Example: github.com/cdvelop/gitgo
func GoGetModulePath() (string, error) {
	file, err := os.Open("go.mod")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module directive not found in go.mod")
}

// goModExists checks if go.mod exists
func goModExists() bool {
	_, err := os.Stat("go.mod")
	return err == nil
}
