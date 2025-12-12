package gitgo

import (
	"errors"
	"fmt"
	"testing"
)

func TestGitHandler_Push(t *testing.T) {
	// Mock the runner to simulate git commands
	mockRunner(t, func(name string, args ...string) (string, error) {
		if name != "git" {
			return "", nil
		}

		// Simulate various git commands
		switch args[0] {
		case "symbolic-ref":
			return "main", nil
		case "rev-parse":
			if args[1] == "--symbolic-full-name" {
				// hasUpstream
				return "origin/main", nil
			}
			if len(args) == 2 && args[1] == "v0.0.1" {
				// tagExists
				return "", nil
			}
		case "push":
			return "", nil
		}
		return "", nil
	})

	h := NewGitHandler()
	if err := h.Push("v0.0.1"); err != nil {
		t.Errorf("Push failed: %v", err)
	}
}

func TestGitHandler_Push_NoUpstream(t *testing.T) {
	mockRunner(t, func(name string, args ...string) (string, error) {
		if name != "git" { return "", nil }
		switch args[0] {
		case "symbolic-ref": return "main", nil
		case "rev-parse":
			if args[1] == "--symbolic-full-name" {
				return "", errors.New("no upstream")
			}
		case "push": return "", nil
		}
		return "", nil
	})

	h := NewGitHandler()
	if err := h.Push(""); err != nil {
		t.Errorf("Push failed: %v", err)
	}
}

func TestWorkflowPush_Unit(t *testing.T) {
	// Capture output
	var capturedLogs []string
	SetLogger(func(v ...any) {
		capturedLogs = append(capturedLogs, fmt.Sprint(v...))
	})

	mockRunner(t, func(name string, args ...string) (string, error) {
		if name == "git" {
			if args[0] == "describe" {
				return "v0.0.1", nil
			}
			if args[0] == "rev-parse" {
				if len(args) > 1 && args[1] == "v0.0.2" {
					return "", errors.New("not found")
				}
			}
		}
		return "", nil
	})

	err := WorkflowPush("message", "")
	if err != nil {
		t.Errorf("WorkflowPush failed: %v", err)
	}
}
