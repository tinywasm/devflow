package gitgo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowGoPush_Unit(t *testing.T) {
    mockRunner(t, func(name string, args ...string) (string, error) {
        if name == "git" {
            if args[0] == "describe" { return "v0.0.1", nil }
             // tag exists check
            if args[0] == "rev-parse" && len(args) == 2 { return "", errors.New("not found") }
        }
        if name == "go" {
            if args[0] == "list" { return "example.com/mod", nil }
            return "", nil
        }
        return "", nil
    })

    err := WorkflowGoPush("msg", "", "")
    if err != nil {
        t.Errorf("WorkflowGoPush failed: %v", err)
    }
}

func TestGoHandler_UpdateDependents_Mocked(t *testing.T) {
    root, _ := os.MkdirTemp("", "gohandler-test-")
    defer os.RemoveAll(root)

    // Main module
    mainMod := filepath.Join(root, "main")
    os.Mkdir(mainMod, 0755)
    os.WriteFile(filepath.Join(mainMod, "go.mod"), []byte("module example.com/main\n\ngo 1.20"), 0644)

    // Dependent module
    depMod := filepath.Join(root, "dep")
    os.Mkdir(depMod, 0755)
    os.WriteFile(filepath.Join(depMod, "go.mod"), []byte("module example.com/dep\n\ngo 1.20\n\nrequire example.com/main v0.0.1"), 0644)

    h := NewGoHandler()

    // Capture logs
	var capturedLogs []string
	SetLogger(func(v ...any) {
		capturedLogs = append(capturedLogs, fmt.Sprint(v...))
	})

    mockRunner(t, func(name string, args ...string) (string, error) {
        // Mock success for go get and go mod tidy
        return "", nil
    })

    err := h.UpdateDependents("example.com/main", "v0.0.2", root)
    if err != nil {
        t.Errorf("UpdateDependents failed: %v", err)
    }

    found := false
    for _, l := range capturedLogs {
        if strings.Contains(l, "Updated:") && strings.Contains(l, depMod) {
            found = true
            break
        }
    }
    if !found {
        t.Errorf("Expected log 'Updated: ...', got %v", capturedLogs)
    }
}
