package gitgo

import (
	"os"
	"testing"
)

func TestFindDependentModules(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Main module
	mainDir := tmpDir + "/main"
	os.MkdirAll(mainDir, 0755)
	// Important: module name matches what dep1 requires
	os.WriteFile(mainDir+"/go.mod", []byte("module github.com/test/main\n\ngo 1.20\n"), 0644)

	// Dependent module 1
	dep1Dir := tmpDir + "/dep1"
	os.MkdirAll(dep1Dir, 0755)
	dep1Mod := `module github.com/test/dep1

go 1.20

require github.com/test/main v0.0.1
`
	os.WriteFile(dep1Dir+"/go.mod", []byte(dep1Mod), 0644)

	// Independent module
	indepDir := tmpDir + "/indep"
	os.MkdirAll(indepDir, 0755)
	os.WriteFile(indepDir+"/go.mod", []byte("module github.com/test/indep\n\ngo 1.20\n"), 0644)

	// Search dependents
	dependents, err := findDependentModules("github.com/test/main", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should find only dep1
	if len(dependents) != 1 {
        for _, d := range dependents {
            t.Logf("Found: %s", d)
        }
		t.Errorf("Expected 1 dependent, got %d", len(dependents))
	}
}

func TestHasDependency(t *testing.T) {
	tmpDir := t.TempDir()
	gomodPath := tmpDir + "/go.mod"

	content := `module github.com/test/repo

go 1.20

require github.com/cdvelop/gitgo v0.0.1
`
	os.WriteFile(gomodPath, []byte(content), 0644)

	// Should find the dependency
	if !hasDependency(gomodPath, "github.com/cdvelop/gitgo") {
		t.Error("Expected to find dependency")
	}

	// Should not find this one
	if hasDependency(gomodPath, "github.com/other/repo") {
		t.Error("Should not find non-existent dependency")
	}
}
