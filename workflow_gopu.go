package gitgo

import (
	"fmt"
)

// WorkflowGoPush executes the Go module update workflow
func WorkflowGoPush(message string, modulePath string, searchPath string) error {
    defer PrintSummary()

	git := NewGitHandler()
	goHandler := NewGoHandler()

    // 1. Verify & Test (Before any changes)
    if err := goHandler.Tidy(); err != nil {
		return fmt.Errorf("tidy failed: %w", err)
	}
    if err := goHandler.Verify(); err != nil {
        return fmt.Errorf("verify failed: %w", err)
    }
    // We run tests to ensure safety
    if err := goHandler.Test(); err != nil {
        return fmt.Errorf("tests failed: %w", err)
    }

	if message == "" {
		message = "update go module"
	}

    if modulePath == "" {
        path, err := goHandler.GetModulePath()
        if err != nil {
             return fmt.Errorf("failed to detect module path: %w", err)
        }
        modulePath = path
        log("Detected module path:", modulePath)
    }

	// 2. Add & Commit
	if err := git.Add(); err != nil {
		return fmt.Errorf("add failed: %w", err)
	}

	if err := git.Commit(message); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	// 3. Tag
	tag, err := git.GenerateNextTag()
	if err != nil {
		return fmt.Errorf("tag generation failed: %w", err)
	}
	if err := git.CreateTag(tag); err != nil {
		log("Warning: tag creation:", err)
	}

	// 4. Push
	if err := git.Push(tag); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// 5. Update Dependents
    if modulePath != "" {
        if err := goHandler.UpdateDependents(modulePath, tag, searchPath); err != nil {
            return fmt.Errorf("update dependents failed: %w", err)
        }
    }

	return nil
}
