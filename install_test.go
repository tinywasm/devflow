package devflow

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstallAllCommands installs all commands from cmd/ directory
// Run with: go test -run TestInstallAllCommands
func TestInstallAllCommands(t *testing.T) {
	// Get project root
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Read cmd/ directory dynamically
	cmdDir := filepath.Join(cwd, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("Failed to read cmd/ directory: %v", err)
	}

	// Collect all subdirectories (commands)
	var commands []string
	for _, entry := range entries {
		if entry.IsDir() {
			commands = append(commands, entry.Name())
		}
	}

	if len(commands) == 0 {
		t.Fatal("No commands found in cmd/ directory")
	}

	for _, cmd := range commands {
		output, err := RunCommand("go", "install", "./cmd/"+cmd)
		if err != nil {
			t.Fatalf("Failed to install %s: %v\nOutput: %s", cmd, err, output)
		}
	}

	t.Log("✅ All commands installed")
}
