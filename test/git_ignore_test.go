package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitIgnoreAdd(t *testing.T) {
	tmp := t.TempDir()
	g, err := devflow.NewGit()
	if err != nil {
		t.Fatalf("failed to create Git: %v", err)
	}
	g.SetRootDir(tmp)

	t.Run("DoesNotCreateGitignoreIfShouldWriteIsFalse", func(t *testing.T) {
		g.SetShouldWrite(func() bool { return false })
		err := g.GitIgnoreAdd(".env")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		path := filepath.Join(tmp, ".gitignore")
		if _, err := os.Stat(path); err == nil {
			t.Error("expected .gitignore NOT to be created when shouldWrite is false")
		}
	})

	t.Run("CreatesGitignoreIfShouldWriteIsTrue", func(t *testing.T) {
		g.SetShouldWrite(func() bool { return true })
		err := g.GitIgnoreAdd(".env")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		path := filepath.Join(tmp, ".gitignore")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		if !strings.Contains(string(content), ".env") {
			t.Errorf("expected .gitignore to contain .env, got %s", string(content))
		}
	})

	t.Run("DoesNotAddDuplicateEntry", func(t *testing.T) {
		g.SetShouldWrite(func() bool { return true })

		// Initial add
		g.GitIgnoreAdd(".env")

		// Second add
		err := g.GitIgnoreAdd(".env")
		if err != nil {
			t.Errorf("expected no error on duplicate add, got %v", err)
		}

		path := filepath.Join(tmp, ".gitignore")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}

		count := strings.Count(string(content), ".env")
		if count != 1 {
			t.Errorf("expected .env to appear once, got %d", count)
		}
	})
}
