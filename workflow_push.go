package gitgo

import (
	"fmt"
)

// WorkflowPush executes the complete push workflow
// Equivalent to the complete logic of pu.sh
//
// Parameters:
//   message: Commit message (required)
//   tag: Optional tag (if empty, auto-generated)
func WorkflowPush(message, tag string) error {
	// Validate message
	if message == "" {
		message = "auto update package"
	}

	// 1. Git add
	if err := GitAdd(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// 2. Commit (only if there are changes)
	if err := GitCommit(message); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	// 3. Determine tag (provided or generated)
	finalTag := tag
	if finalTag == "" {
		generatedTag, err := GitGenerateNextTag()
		if err != nil {
			return fmt.Errorf("failed to generate tag: %w", err)
		}
		finalTag = generatedTag
	}

	// 4. Create tag
	if err := GitCreateTag(finalTag); err != nil {
		// If it already exists, not fatal error (can be re-run)
		log("Warning:", err)
	}

	// 5. Push commits and tag
	if err := GitPushWithTags(finalTag); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	log("Push completed:", finalTag)
	return nil
}
