package devflow

import (
	"testing"
)

func TestGo_SetLog(t *testing.T) {
	git := NewGit()
	g := NewGo(git)

	// Test that SetLog works
	called := false
	g.SetLog(func(args ...any) {
		called = true
	})

	// Call log to verify it works
	g.log("test")

	if !called {
		t.Error("Expected log function to be called")
	}
}

func TestGo_NewGo(t *testing.T) {
	git := NewGit()
	g := NewGo(git)

	if g == nil {
		t.Error("Expected NewGo to return non-nil")
	}

	if g.git != git {
		t.Error("Expected git handler to be set")
	}
}
