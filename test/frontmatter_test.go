package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    devflow.PlanMeta
		wantErr error
	}{
		{
			name:    "valid PLAN and tag",
			content: "---\nPLAN: \"feat: something\"\ntag: v1.0.0\n---\nBody",
			want:    devflow.PlanMeta{Message: "feat: something", Tag: "v1.0.0"},
		},
		{
			name:    "valid PLAN only",
			content: "---\nPLAN: just message\n---\n",
			want:    devflow.PlanMeta{Message: "just message"},
		},
		{
			name:    "missing opening fence",
			content: "PLAN: no fence\n---\n",
			wantErr: devflow.ErrFrontmatterMissing,
		},
		{
			name:    "unclosed fence",
			content: "---\nPLAN: no end\n",
			wantErr: devflow.ErrFrontmatterUnclosed,
		},
		{
			name:    "missing PLAN",
			content: "---\ntag: v1.0.0\n---\n",
			wantErr: devflow.ErrFrontmatterNoPlan,
		},
		{
			name:    "legacy message key rejected",
			content: "---\nmessage: old\n---\n",
			wantErr: devflow.ErrFrontmatterNoPlan,
		},
		{
			name:    "with single quotes",
			content: "---\nPLAN: 'quoted'\n---\n",
			want:    devflow.PlanMeta{Message: "quoted"},
		},
		{
			name:    "unknown keys ignored",
			content: "---\nPLAN: ok\nunknown: value\n---\n",
			want:    devflow.PlanMeta{Message: "ok"},
		},
		{
			name:    "CRLF support",
			content: "---\r\nPLAN: crlf\r\n---\r\n",
			want:    devflow.PlanMeta{Message: "crlf"},
		},
		{
			name:    "blank lines internal",
			content: "---\n\nPLAN: spaced\n\n---\n",
			want:    devflow.PlanMeta{Message: "spaced"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := devflow.ParseFrontmatter(tt.content)
			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Errorf("ParseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFrontmatter() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseFrontmatter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadPlanMeta(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "PLAN.md")
	content := "---\nPLAN: from file\n---\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := devflow.ReadPlanMeta(path)
	if err != nil {
		t.Fatalf("ReadPlanMeta failed: %v", err)
	}
	if meta.Message != "from file" {
		t.Errorf("expected 'from file', got %q", meta.Message)
	}
}
