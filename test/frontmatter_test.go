package devflow_test

import "github.com/tinywasm/devflow"

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	planKey := devflow.FrontmatterKeyPlan
	tagKey := devflow.FrontmatterKeyTag

	tests := []struct {
		name    string
		content string
		want    devflow.PlanMeta
		wantErr error
	}{
		{
			name:    "valid PLAN and tag",
			content: fmt.Sprintf("---\n%s: \"feat: something\"\n%s: v1.0.0\n---\nBody", planKey, tagKey),
			want:    devflow.PlanMeta{Message: "feat: something", Tag: "v1.0.0"},
		},
		{
			name:    "valid PLAN only",
			content: fmt.Sprintf("---\n%s: just message\n---\n", planKey),
			want:    devflow.PlanMeta{Message: "just message"},
		},
		{
			name:    "missing opening fence",
			content: fmt.Sprintf("%s: no fence\n---\n", planKey),
			wantErr: devflow.ErrFrontmatterMissing,
		},
		{
			name:    "unclosed fence",
			content: fmt.Sprintf("---\n%s: no end\n", planKey),
			wantErr: devflow.ErrFrontmatterUnclosed,
		},
		{
			name:    "missing PLAN",
			content: fmt.Sprintf("---\n%s: v1.0.0\n---\n", tagKey),
			wantErr: devflow.ErrFrontmatterNoPlan,
		},
		{
			name:    "legacy message key rejected",
			content: "---\nmessage: old\n---\n",
			wantErr: devflow.ErrFrontmatterNoPlan,
		},
		{
			name:    "with single quotes",
			content: fmt.Sprintf("---\n%s: 'quoted'\n---\n", planKey),
			want:    devflow.PlanMeta{Message: "quoted"},
		},
		{
			name:    "unknown keys ignored",
			content: fmt.Sprintf("---\n%s: ok\nunknown: value\n---\n", planKey),
			want:    devflow.PlanMeta{Message: "ok"},
		},
		{
			name:    "CRLF support",
			content: fmt.Sprintf("---\r\n%s: crlf\r\n---\r\n", planKey),
			want:    devflow.PlanMeta{Message: "crlf"},
		},
		{
			name:    "blank lines internal",
			content: fmt.Sprintf("---\n\n%s: spaced\n\n---\n", planKey),
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
	content := fmt.Sprintf("---\n%s: from file\n---\n", devflow.FrontmatterKeyPlan)
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
