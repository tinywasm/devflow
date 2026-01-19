package devflow

import (
	"os"
	"path/filepath"
	"strings"
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

func TestGoModFile(t *testing.T) {
	tmp := t.TempDir()
	gomodPath := filepath.Join(tmp, "go.mod")

	t.Run("RemoveReplace_Inline", func(t *testing.T) {
		content := `module test
go 1.20
require github.com/test/lib v1.0.0
replace github.com/test/lib => ../lib
`
		os.WriteFile(gomodPath, []byte(content), 0644)

		gm, err := NewGoModFile(gomodPath)
		if err != nil {
			t.Fatal(err)
		}

		removed := gm.RemoveReplace("github.com/test/lib")
		if !removed {
			t.Error("expected replace to be removed")
		}
		if !gm.modified {
			t.Error("expected modified to be true")
		}

		err = gm.Save()
		if err != nil {
			t.Fatal(err)
		}

		newContent, _ := os.ReadFile(gomodPath)
		if strings.Contains(string(newContent), "replace github.com/test/lib") {
			t.Error("replace directive still exists in file")
		}
	})

	t.Run("RemoveReplace_Block", func(t *testing.T) {
		content := `module test
replace (
	github.com/test/lib => ../lib
	github.com/test/other => ../other
)
`
		os.WriteFile(gomodPath, []byte(content), 0644)

		gm, err := NewGoModFile(gomodPath)
		if err != nil {
			t.Fatal(err)
		}

		gm.RemoveReplace("github.com/test/lib")
		gm.Save()

		newContent, _ := os.ReadFile(gomodPath)
		if strings.Contains(string(newContent), "github.com/test/lib") {
			t.Error("replace directive still exists in block")
		}
		if !strings.Contains(string(newContent), "github.com/test/other") {
			t.Error("expected other replace to remain")
		}
	})

	t.Run("RemoveReplace_EmptyBlock", func(t *testing.T) {
		content := `module test
replace (
	github.com/test/lib => ../lib
)
`
		os.WriteFile(gomodPath, []byte(content), 0644)

		gm, err := NewGoModFile(gomodPath)
		if err != nil {
			t.Fatal(err)
		}

		gm.RemoveReplace("github.com/test/lib")
		gm.Save()

		newContent, _ := os.ReadFile(gomodPath)
		if strings.Contains(string(newContent), "replace (") {
			t.Error("replace block should have been removed when empty")
		}
	})

	t.Run("HasOtherReplaces", func(t *testing.T) {
		content := `module test
replace github.com/test/lib => ../lib
replace github.com/test/other => ../other
`
		gm, _ := NewGoModFile(gomodPath)
		gm.lines = strings.Split(content, "\n")

		if !gm.HasOtherReplaces("github.com/test/lib") {
			t.Error("expected true when other replaces exist")
		}

		if gm.HasOtherReplaces("") {
			if !gm.HasOtherReplaces("non-existent") {
				t.Error("expected true when any replace exists")
			}
		}

		gm.lines = []string{"module test", "go 1.20"}
		if gm.HasOtherReplaces("") {
			t.Error("expected false when no replaces exist")
		}
	})
}
