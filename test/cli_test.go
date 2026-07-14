package devflow_test

import (
	"testing"

	"github.com/tinywasm/devflow"
)

func TestParseCLIArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantMsg      string
		wantTag      string
		wantIsHelp   bool
		wantIsRelease bool
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
			name:         "Message only",
			args:         []string{"cmd", "feat: something"},
			wantMsg:      "feat: something",
			wantTag:      "",
			wantIsHelp:   false,
			wantIsRelease: false,
		},
		{
			name:         "Message and tag",
			args:         []string{"cmd", "feat: something", "v1.2.3"},
			wantMsg:      "feat: something",
			wantTag:      "v1.2.3",
			wantIsHelp:   false,
			wantIsRelease: false,
		},
		{
			name:         "Empty args",
			args:         []string{"cmd"},
			wantMsg:      "",
			wantTag:      "",
			wantIsHelp:   false,
			wantIsRelease: false,
		},
		{
			name:         "Message with -release flag",
			args:         []string{"cmd", "feat: something", "-release"},
			wantMsg:      "feat: something",
			wantTag:      "",
			wantIsHelp:   false,
			wantIsRelease: true,
		},
		{
			name:         "Message, tag, and -release flag",
			args:         []string{"cmd", "feat: something", "v1.2.3", "-release"},
			wantMsg:      "feat: something",
			wantTag:      "v1.2.3",
			wantIsHelp:   false,
			wantIsRelease: true,
		},
		{
			name:         "-release at different position",
			args:         []string{"cmd", "-release", "feat: something", "v1.2.3"},
			wantMsg:      "-release",
			wantTag:      "feat: something",
			wantIsHelp:   false,
			wantIsRelease: true,
		},
		{
			name:         "--release flag variant",
			args:         []string{"cmd", "feat: something", "--release"},
			wantMsg:      "feat: something",
			wantTag:      "",
			wantIsHelp:   false,
			wantIsRelease: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, tag, isHelp, isRelease := devflow.ParseCLIArgs(tt.args)
			if msg != tt.wantMsg {
				t.Errorf("ParseCLIArgs() msg = %v, want %v", msg, tt.wantMsg)
			}
			if tag != tt.wantTag {
				t.Errorf("ParseCLIArgs() tag = %v, want %v", tag, tt.wantTag)
			}
			if isHelp != tt.wantIsHelp {
				t.Errorf("ParseCLIArgs() isHelp = %v, want %v", isHelp, tt.wantIsHelp)
			}
			if isRelease != tt.wantIsRelease {
				t.Errorf("ParseCLIArgs() isRelease = %v, want %v", isRelease, tt.wantIsRelease)
			}
		})
	}
}

func TestParseCLIArgs_NoCascadeFlag(t *testing.T) {
	// Simulated main logic for flag filtering
	filter := func(args []string) (bool, []string) {
		var noCascade bool
		filtered := []string{args[0]}
		for _, arg := range args[1:] {
			if arg == "--no-cascade" {
				noCascade = true
			} else {
				filtered = append(filtered, arg)
			}
		}
		return noCascade, filtered
	}

	args := []string{"gopush", "feat: test", "--no-cascade"}
	noCascade, filtered := filter(args)
	if !noCascade {
		t.Fatal("expected noCascade to be true")
	}
	msg, _, _, _ := devflow.ParseCLIArgs(filtered)
	if msg != "feat: test" {
		t.Errorf("expected message 'feat: test', got %q", msg)
	}

	// Absent case
	args = []string{"gopush", "feat: test"}
	noCascade, _ = filter(args)
	if noCascade {
		t.Fatal("expected noCascade to be false")
	}
}

func TestParseReleaseArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
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
			name:       "Help - -h",
			args:       []string{"cmd", "-h"},
			wantIsHelp: true,
		},
		{
			name:       "Tag provided",
			args:       []string{"cmd", "v1.2.3"},
			wantTag:    "v1.2.3",
			wantIsHelp: false,
		},
		{
			name:       "No tag",
			args:       []string{"cmd"},
			wantTag:    "",
			wantIsHelp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, isHelp := devflow.ParseReleaseArgs(tt.args)
			if tag != tt.wantTag {
				t.Errorf("ParseReleaseArgs() tag = %v, want %v", tag, tt.wantTag)
			}
			if isHelp != tt.wantIsHelp {
				t.Errorf("ParseReleaseArgs() isHelp = %v, want %v", isHelp, tt.wantIsHelp)
			}
		})
	}
}
