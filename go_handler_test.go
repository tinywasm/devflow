package gitgo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoHandler_Tidy(t *testing.T) {
	dir, cleanup := testCreateGoModule("example.com/test")
	defer cleanup()

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	h := NewGoHandler()
	if err := h.Tidy(); err != nil {
		t.Errorf("Tidy failed: %v", err)
	}
}

func TestGoHandler_FindDependentModules(t *testing.T) {
	// Create a structure:
	// root/
	//   lib/ (main module)
	//   app1/ (depends on lib)
	//   app2/ (no dependency)

	root, _ := os.MkdirTemp("", "gitgo-deps-")
	defer os.RemoveAll(root)

	libPath := filepath.Join(root, "lib")
	os.Mkdir(libPath, 0755)
	os.WriteFile(filepath.Join(libPath, "go.mod"), []byte("module example.com/lib\n\ngo 1.20"), 0644)

	app1Path := filepath.Join(root, "app1")
	os.Mkdir(app1Path, 0755)
	os.WriteFile(filepath.Join(app1Path, "go.mod"), []byte("module example.com/app1\n\ngo 1.20\n\nrequire example.com/lib v0.0.1"), 0644)

	app2Path := filepath.Join(root, "app2")
	os.Mkdir(app2Path, 0755)
	os.WriteFile(filepath.Join(app2Path, "go.mod"), []byte("module example.com/app2\n\ngo 1.20"), 0644)

	h := NewGoHandler()
	deps, err := h.findDependentModules("example.com/lib", root)
	if err != nil {
		t.Fatal(err)
	}

	if len(deps) != 1 {
		t.Errorf("Expected 1 dependent, got %d", len(deps))
	} else {
		if deps[0] != app1Path {
			t.Errorf("Expected dependent %s, got %s", app1Path, deps[0])
		}
	}
}
