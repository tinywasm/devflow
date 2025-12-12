package gitgo

import (
	"fmt"
)

// WorkflowGoPU executes the complete workflow for Go projects
// Equivalent to the complete logic of gopu.sh
//
// Parameters:
//   message: Commit message
//   tag: Optional tag
//   skipTests: If true, skips tests
//   skipRace: If true, skips race tests
//   searchPath: Path to search for dependent modules (default: "..")
func WorkflowGoPU(message, tag string, skipTests, skipRace bool, searchPath string) error {
	// Default values
	if message == "" {
		message = "auto update Go package"
	}

	if searchPath == "" {
		searchPath = ".."
	}

	// 1. Verify go.mod
	log("Verifying Go module...")
	if err := GoModVerify(); err != nil {
		return fmt.Errorf("go mod verify failed: %w", err)
	}

	// 2. Run tests (if not skipped)
	if !skipTests {
		if err := GoTest(); err != nil {
			return fmt.Errorf("tests failed: %w", err)
		}
	}

	// 3. Run race tests (if not skipped)
	if !skipRace && !skipTests {
		if err := GoTestRace(); err != nil {
			return fmt.Errorf("race tests failed: %w", err)
		}
	}

	// 4. Execute push workflow
	log("Executing push workflow...")
	if err := WorkflowPush(message, tag); err != nil {
		return fmt.Errorf("push workflow failed: %w", err)
	}

	// 5. Get created tag
	latestTag, err := GitGetLatestTag()
	if err != nil {
		log("Warning: could not get latest tag:", err)
		return nil // Not fatal error
	}

	// 6. Get module name
	modulePath, err := GoGetModulePath()
	if err != nil {
		log("Warning: could not get module path:", err)
		return nil
	}

	// 7. Update dependent modules
	log("Updating dependent modules...")
	if err := GoUpdateDependents(modulePath, latestTag, searchPath); err != nil {
		log("Warning: failed to update dependents:", err)
		// Not fatal error
	}

	log("GoPU completed:", latestTag)
	return nil
}
