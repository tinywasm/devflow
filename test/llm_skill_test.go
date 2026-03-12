package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLLM_GetMasterContent(t *testing.T) {
	llm := devflow.NewLLM()
	content, err := llm.GetMasterContent()
	if err != nil {
		t.Fatalf("failed to get master content: %v", err)
	}
	
	expected := "Skills location: ~/tinywasm/skills/"
	if !strings.Contains(content, expected) {
		t.Errorf("expected content to contain %q, got %q", expected, content)
	}
}

func TestLLM_InstallSkills(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Simular home directory temporal
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	llm := devflow.NewLLM()
	if err := llm.InstallSkills(); err != nil {
		t.Fatalf("failed to install skills: %v", err)
	}

	// Verificar directorios creados
	skillsRoot := filepath.Join(tmpDir, "tinywasm", "skills")
	requiredSkills := []string{
		"core-principles",
		"testing",
		"documentation",
		"wasm",
		"agents-workflow",
		"dev-protocols",
	}

	for _, skill := range requiredSkills {
		skillPath := filepath.Join(skillsRoot, skill, "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			t.Errorf("skill file not found: %s", skillPath)
		}
	}
}

func TestLLM_DetectInstalledLLMs(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	llm := devflow.NewLLM()

	// Caso 1: No hay LLMs instalados
	installed := llm.DetectInstalledLLMs()
	if len(installed) != 0 {
		t.Errorf("expected 0 LLMs, got %d", len(installed))
	}

	// Caso 2: Solo Claude instalado
	claudeDir := filepath.Join(tmpDir, ".claude")
	os.Mkdir(claudeDir, 0755)

	installed = llm.DetectInstalledLLMs()
	if len(installed) != 1 {
		t.Fatalf("expected 1 LLM, got %d", len(installed))
	}
	if installed[0].Name != "claude" {
		t.Errorf("expected 'claude', got '%s'", installed[0].Name)
	}
}

func TestLLM_Sync(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Simular instalación de LLM
	claudeDir := filepath.Join(tmpDir, ".claude")
	os.Mkdir(claudeDir, 0755)
	claudeConfig := filepath.Join(claudeDir, "CLAUDE.md")
	initialContent := "# Existing Config\n"
	os.WriteFile(claudeConfig, []byte(initialContent), 0644)

	llm := devflow.NewLLM()
	summary, err := llm.Sync("", false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if !strings.Contains(summary, "Config updated") {
		t.Errorf("summary should mention updated config: %s", summary)
	}

	// Verificar que se instalaron skills
	skillsRoot := filepath.Join(tmpDir, "tinywasm", "skills")
	if _, err := os.Stat(skillsRoot); os.IsNotExist(err) {
		t.Error("skills dir not created during Sync")
	}

	// Verificar que se añadió la línea al config
	content, _ := os.ReadFile(claudeConfig)
	if !strings.Contains(string(content), "Skills location: ~/tinywasm/skills/") {
		t.Error("reference line not added to config")
	}
	if !strings.Contains(string(content), initialContent) {
		t.Error("initial content lost")
	}

	// Segunda ejecución: debe skipear
	summary2, _ := llm.Sync("", false)
	if !strings.Contains(summary2, "already had reference") {
		t.Errorf("summary should mention skipped config: %s", summary2)
	}
}

func TestLLM_Sync_SpecificLLM(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)
	os.Mkdir(filepath.Join(tmpDir, ".gemini"), 0755)

	llm := devflow.NewLLM()
	summary, _ := llm.Sync("claude", false)

	if !strings.Contains(summary, "claude") {
		t.Error("summary should mention claude")
	}
	if strings.Contains(summary, "gemini") {
		t.Error("summary should NOT mention gemini")
	}

	// Verificar Claude creado, Gemini no
	if _, err := os.Stat(filepath.Join(tmpDir, ".claude", "CLAUDE.md")); os.IsNotExist(err) {
		t.Error("claude config not created")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".gemini", "GEMINI.md")); err == nil {
		t.Error("gemini config should not be created")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	content := "Test content"
	os.WriteFile(src, []byte(content), 0644)

	if err := devflow.CopyFile(src, dst); err != nil {
		t.Fatalf("devflow.CopyFile failed: %v", err)
	}

	dstContent, _ := os.ReadFile(dst)
	if string(dstContent) != content {
		t.Errorf("content mismatch")
	}
}
