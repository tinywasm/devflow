package devflow

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
			if err := ValidateCommitMessage(tt.message); (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommitMessage() error = %v, wantErr %v", err, tt.wantErr)
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
			if got := FormatCommitMessage(tt.message); got != tt.want {
				t.Errorf("FormatCommitMessage() = %v, want %v", got, tt.want)
			}
		})
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
			got := ValidateShellSafeMessage(tt.message)
			if (got != "") != tt.want {
				t.Errorf("ValidateShellSafeMessage() = %v, want warning? %v", got, tt.want)
			}
		})
	}
}
