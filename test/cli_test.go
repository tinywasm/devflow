package devflow_test

import (
	"testing"

	"github.com/tinywasm/devflow"
)

func TestParseCLIArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantMsg    string
		wantTag    string
		wantIsHelp bool
	}{
		{
			name:       "Help - help",
			args:       []string{"cmd", "help"},
			wantIsHelp: true,
		},
		{
			name:       "Help - -help",
			args:       []string{"cmd", "-help"},
			wantIsHelp: true,
		},
		{
			name:       "Help - --help",
			args:       []string{"cmd", "--help"},
			wantIsHelp: true,
		},
		{
			name:       "Help - h",
			args:       []string{"cmd", "h"},
			wantIsHelp: true,
		},
		{
			name:       "Help - -h",
			args:       []string{"cmd", "-h"},
			wantIsHelp: true,
		},
		{
			name:       "Help - ?",
			args:       []string{"cmd", "?"},
			wantIsHelp: true,
		},
		{
			name:       "Help - -?",
			args:       []string{"cmd", "-?"},
			wantIsHelp: true,
		},
		{
			name:       "Message only",
			args:       []string{"cmd", "feat: something"},
			wantMsg:    "feat: something",
			wantTag:    "",
			wantIsHelp: false,
		},
		{
			name:       "Message and tag",
			args:       []string{"cmd", "feat: something", "v1.2.3"},
			wantMsg:    "feat: something",
			wantTag:    "v1.2.3",
			wantIsHelp: false,
		},
		{
			name:       "Empty args",
			args:       []string{"cmd"},
			wantMsg:    "",
			wantTag:    "",
			wantIsHelp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, tag, isHelp := devflow.ParseCLIArgs(tt.args)
			if msg != tt.wantMsg {
				t.Errorf("ParseCLIArgs() msg = %v, want %v", msg, tt.wantMsg)
			}
			if tag != tt.wantTag {
				t.Errorf("ParseCLIArgs() tag = %v, want %v", tag, tt.wantTag)
			}
			if isHelp != tt.wantIsHelp {
				t.Errorf("ParseCLIArgs() isHelp = %v, want %v", isHelp, tt.wantIsHelp)
			}
		})
	}
}
