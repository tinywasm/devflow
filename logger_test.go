package gitgo

import (
	"testing"
)

func TestSetLogger(t *testing.T) {
	// Capture logs
	var logged []any

	customLog := func(v ...any) {
		logged = append(logged, v...)
	}

	SetLogger(customLog)

	log("test", "message")

	if len(logged) != 2 {
		t.Errorf("Expected 2 logged items, got %d", len(logged))
	}

	if logged[0] != "test" {
		t.Errorf("Expected 'test', got %v", logged[0])
	}
}
