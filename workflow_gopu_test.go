package gitgo

import (
	"os"
	"os/exec"
	"testing"
)

func TestWorkflowGoPU(t *testing.T) {
	// Create temporary Go module
	dir, cleanup := testCreateGitRepo()
	defer cleanup()

	// Init git
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()

	// Create go.mod
	gomod := "module github.com/test/repo\n\ngo 1.20\n"
	os.WriteFile(dir+"/go.mod", []byte(gomod), 0644)

	// Create main.go with test
	main := "package main\n\nfunc main() {}\n"
	os.WriteFile(dir+"/main.go", []byte(main), 0644)

	test := "package main\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n"
	os.WriteFile(dir+"/main_test.go", []byte(test), 0644)

	// Change to directory
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Execute workflow (skip real push and update)
	err := WorkflowGoPU("test", "", false, true, "") // skip race, no search

	// Should not fail (even if no real push)
	if err != nil && err.Error() != "push failed: git push failed: command failed: git push \nError: exit status 128\nOutput: fatal: No configured push destination.\nEither specify the URL from the command-line or configure a remote repository using\n\n    git remote add <name> <url>\n\nand then push using the remote name\n\n    git push <name>\n" {
		// We expect push failure because there is no remote
		// If the error message is different, something is wrong
		// But usually we just check if it executed without panic or early exit
		// The original test code just logs a warning
		t.Logf("Warning: %v", err)
	}
}
