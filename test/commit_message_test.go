package devflow_test

import "github.com/tinywasm/devflow"

import (
	"testing"
)

func TestValidateCommitMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{"Valid message", "feat: added validation", false},
		{"Message with spaces", "  feat: added validation  ", false},
		{"Empty message", "", true},
		{"Whitespace only", "   ", true},
		{"Message with backticks", "feat: added `afterLine` parameter", false},
		{"Message with double quotes", "feat: said \"hello\"", false},
		{"Multiline message", "feat: first line\n\n- second line\n- third line", false},
		{"Internal single quotes", "docs: don't forget the readme", false},
		{"Wrapped in single quotes", "'feat: some feature'", false},
		{"Message with escaped single quote", "feat: handled\\'s item", false},
		{"Complex single quote usage", "feat: it's a 'complex' task", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := devflow.ValidateCommitMessage(tt.message); (err != nil) != tt.wantErr {
				t.Errorf("devflow.ValidateCommitMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatCommitMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"Normal message", "feat: test", "feat: test"},
		{"Needs trimming", "  feat: test  ", "feat: test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := devflow.FormatCommitMessage(tt.message); got != tt.want {
				t.Errorf("devflow.FormatCommitMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildDepsCommitMessage defines the deterministic commit message contract
// for cascade dep bumps (PLAN.md Fase 2). No AI involved: the semantic content
// of a dep-bump commit is fully known by construction. The `cause:` line
// propagates the root gopush message through the whole cascade for
// traceability.
func TestBuildDepsCommitMessage(t *testing.T) {
	tests := []struct {
		name      string
		bumps     []devflow.DepBump
		rootCause string
		want      string
	}{
		{
			name: "single bump no cause",
			bumps: []devflow.DepBump{
				{ModulePath: "github.com/tinywasm/router", OldVersion: "v0.1.2", NewVersion: "v0.1.3"},
			},
			rootCause: "",
			want:      "deps: update router to v0.1.3\n\n- github.com/tinywasm/router v0.1.2 → v0.1.3",
		},
		{
			name: "single bump with cause",
			bumps: []devflow.DepBump{
				{ModulePath: "github.com/tinywasm/router", OldVersion: "v0.1.2", NewVersion: "v0.1.3"},
			},
			rootCause: "feat: rutas con parámetros opcionales",
			want:      "deps: update router to v0.1.3\n\ncause: feat: rutas con parámetros opcionales\n\n- github.com/tinywasm/router v0.1.2 → v0.1.3",
		},
		{
			name: "multiple bumps with cause",
			bumps: []devflow.DepBump{
				{ModulePath: "github.com/tinywasm/router", OldVersion: "v0.1.2", NewVersion: "v0.1.3"},
				{ModulePath: "github.com/tinywasm/sse", OldVersion: "v0.1.9", NewVersion: "v0.2.0"},
			},
			rootCause: "fix: escape de headers",
			want:      "deps: update router, sse\n\ncause: fix: escape de headers\n\n- github.com/tinywasm/router v0.1.2 → v0.1.3\n- github.com/tinywasm/sse v0.1.9 → v0.2.0",
		},
		{
			name: "unknown old version omits it",
			bumps: []devflow.DepBump{
				{ModulePath: "github.com/tinywasm/router", NewVersion: "v0.1.3"},
			},
			rootCause: "",
			want:      "deps: update router to v0.1.3\n\n- github.com/tinywasm/router v0.1.3",
		},
		{
			name:      "no bumps yields empty message",
			bumps:     nil,
			rootCause: "feat: irrelevant",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := devflow.BuildDepsCommitMessage(tt.bumps, tt.rootCause)
			if got != tt.want {
				t.Errorf("BuildDepsCommitMessage() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// The prefixes are shared constants between producer (cascade) and any
// consumer that parses logs — string literals in logic are forbidden.
func TestDepsCommitConstants(t *testing.T) {
	if devflow.DepsCommitPrefix != "deps: " {
		t.Errorf("DepsCommitPrefix must be %q, got %q", "deps: ", devflow.DepsCommitPrefix)
	}
	if devflow.CauseLinePrefix != "cause: " {
		t.Errorf("CauseLinePrefix must be %q, got %q", "cause: ", devflow.CauseLinePrefix)
	}
}

func TestValidateShellSafeMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool // true if warning expected
	}{
		{"Safe message", "feat: test", false},
		{"Backticks", "feat: `test`", true},
		{"Dollar sign", "feat: $var", true},
		{"Single quote", "don't forget", true},
		{"Double quotes ok", "said \"hello\"", false},
		{"Mixed backtick and single quote", "don't use `this`", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := devflow.ValidateShellSafeMessage(tt.message)
			if (got != "") != tt.want {
				t.Errorf("devflow.ValidateShellSafeMessage() = %v, want warning? %v", got, tt.want)
			}
		})
	}
}
