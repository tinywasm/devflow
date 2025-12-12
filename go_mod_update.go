package gitgo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoUpdateDependents updates modules that depend on the current one
// Rewrite in Go of gomodtagupdate.sh (much faster)
//
// Parameters:
//   modulePath: Current module path (eg: github.com/cdvelop/gitgo)
//   version: New version/tag (eg: v0.0.5)
//   searchPath: Directory to search for dependent modules (eg: "..")
func GoUpdateDependents(modulePath, version, searchPath string) error {
	if searchPath == "" {
		searchPath = ".."
	}

	log("Searching dependent modules of", modulePath)

	// Find modules that depend on current
	dependents, err := findDependentModules(modulePath, searchPath)
	if err != nil {
		return err
	}

	if len(dependents) == 0 {
		log("No dependent modules found")
		return nil
	}

	log(fmt.Sprintf("Found %d dependent modules", len(dependents)))

	// Update each dependent
	updated := 0
	for _, depDir := range dependents {
		if err := updateModule(depDir, modulePath, version); err != nil {
			log("Error updating", depDir, ":", err)
			continue
		}
		updated++
		log("Updated:", depDir)
	}

	log(fmt.Sprintf("Updated %d modules", updated))
	return nil
}

// findDependentModules searches for modules that have modulePath as dependency
func findDependentModules(modulePath, searchPath string) ([]string, error) {
	var dependents []string

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue despite errors
		}

		// Only go.mod files
		if info.Name() != "go.mod" {
			return nil
		}

		// Don't process current directory's go.mod
		// We are not checking if we are processing the current module or not.
		// In the test, we are searching in a temp dir which contains multiple modules.
		// One of them is the "main" module which is the dependency of "dep1".
		// But "main" module itself might be seen as having dependency if we are not careful?
		// No, "main" module does not depend on "main".

		// Check if this go.mod has the dependency
		if hasDependency(path, modulePath) {
            // Ensure we are not finding the module itself as a dependent
            if filepath.Dir(path) == modulePath {
                // This check is tricky because modulePath in argument is the import path (github.com/...),
                // not the file path.
            }
            // Better check: does the file content contain "module <modulePath>"?
            // If so, it's the module itself (or a module with same name), not a dependent.
            // But wait, hasDependency checks if the file contains the string modulePath.
            // If the file is "module github.com/test/main", it contains "github.com/test/main".
            // So we need to be more specific in hasDependency or here.

            // Let's verify if the match is a "require" or "module" directive.
			dependents = append(dependents, filepath.Dir(path))
		}

		return nil
	})

	return dependents, err
}

// hasDependency checks if a go.mod contains a specific dependency
func hasDependency(gomodPath, modulePath string) bool {
	content, err := os.ReadFile(gomodPath)
	if err != nil {
		return false
	}

	// Search for module path in content, but ignore the "module" directive
    lines := strings.Split(string(content), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "module ") {
            if strings.TrimSpace(strings.TrimPrefix(line, "module")) == modulePath {
                // This is the module itself, not a dependent
                return false
            }
        }
        // Check for dependency.
        // It should be either "require <modulePath> <version>" or inside a require block.
        // We look for word boundaries around modulePath.
        // Example: "require github.com/foo/bar v1.0.0"
        // or just "github.com/foo/bar v1.0.0" (inside block)

        if strings.HasPrefix(line, "module ") {
            continue
        }

        // Simple tokenization
        fields := strings.Fields(line)
        for _, field := range fields {
            if field == modulePath {
                return true
            }
        }
    }

	return false
}

// updateModule updates a specific module to a new version
func updateModule(moduleDir, dependency, version string) error {
	// Change to module directory
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(moduleDir); err != nil {
		return err
	}

	// go get -u dependency@version
	target := fmt.Sprintf("%s@%s", dependency, version)
	cmd := exec.Command("go", "get", "-u", target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go get failed: %w", err)
	}

	// go mod tidy
	cmd = exec.Command("go", "mod", "tidy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}
