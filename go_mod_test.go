package devflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModExistsInCurrentOrParent(t *testing.T) {
	tmp := t.TempDir()

	// Create a subdirectory for the "current" dir
	currentDir := filepath.Join(tmp, "subdir")
	if err := os.Mkdir(currentDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	g, err := NewGo(nil)
	if err != nil {
		t.Fatalf("failed to create Go handler: %v", err)
	}
	g.SetRootDir(currentDir)

	t.Run("ReturnsFalseWhenNoGoModExists", func(t *testing.T) {
		if g.ModExistsInCurrentOrParent() {
			t.Error("expected false when no go.mod exists")
		}
	})

	t.Run("ReturnsTrueWhenGoModInCurrentDir", func(t *testing.T) {
		goModPath := filepath.Join(currentDir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module test"), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}
		defer os.Remove(goModPath)

		if !g.ModExistsInCurrentOrParent() {
			t.Error("expected true when go.mod exists in current dir")
		}
	})

	t.Run("ReturnsTrueWhenGoModInParentDir", func(t *testing.T) {
		goModPath := filepath.Join(tmp, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module test-parent"), 0644); err != nil {
			t.Fatalf("failed to create go.mod in parent: %v", err)
		}
		defer os.Remove(goModPath)

		if !g.ModExistsInCurrentOrParent() {
			t.Error("expected true when go.mod exists in parent dir")
		}
	})
}
