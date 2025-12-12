package gitgo

import (
	"fmt"
)

// WorkflowPush executes the complete push workflow
func WorkflowPush(message, tag string) error {
	defer PrintSummary()

	git := NewGitHandler()

	// Validate message
	if message == "" {
		message = "auto update package"
	}

	// 1. Git add
	if err := git.Add(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// 2. Commit
	if err := git.Commit(message); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	// 3. Determine tag
	finalTag := tag
	if finalTag == "" {
		generatedTag, err := git.GenerateNextTag()
		if err != nil {
			return fmt.Errorf("failed to generate tag: %w", err)
		}
		finalTag = generatedTag
	}

	// 4. Create tag
	if err := git.CreateTag(finalTag); err != nil {
		log("Warning: failed to create tag (might exist):", err)
	}

	// 5. Push
	if err := git.Push(finalTag); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	log("Push completed:", finalTag)
	return nil
}
