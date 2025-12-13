package devflow

import (
	"testing"
)

func TestGetBadgeColor(t *testing.T) {
	tests := []struct {
		typ   string
		value string
		want  string
	}{
		{"license", "MIT", "#007acc"},
		{"go", "1.20", "#007acc"},
		{"tests", "Passing", "#4c1"},
		{"tests", "Failed", "#e05d44"},
		{"coverage", "85", "#4c1"},
		{"coverage", "65", "#dfb317"},
		{"coverage", "30", "#fe7d37"},
		{"coverage", "0", "#e05d44"},
		{"race", "Clean", "#4c1"},
		{"race", "Detected", "#e05d44"},
		{"vet", "OK", "#4c1"},
		{"vet", "Issues", "#e05d44"},
	}

	for _, tt := range tests {
		got := getBadgeColor(tt.typ, tt.value)
		if got != tt.want {
			t.Errorf("getBadgeColor(%q, %q) = %q, want %q", tt.typ, tt.value, got, tt.want)
		}
	}
}

func TestGetGoVersion(t *testing.T) {
	version := getGoVersion()
	if version == "" {
		t.Error("Expected non-empty go version")
	}
}
