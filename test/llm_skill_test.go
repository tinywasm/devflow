package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLLM_InstallSkills(t *testing.T) {
	tmpDir := t.TempDir()

	// Simular home directory temporal
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	llm := devflow.NewLLM()
	if _, err := llm.InstallSkills(); err != nil {
		t.Fatalf("failed to install skills: %v", err)
	}

	// Verificar directorios creados
	skillsRoot := filepath.Join(tmpDir, "skills")
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

	llm := devflow.NewLLM()
	summary, err := llm.Sync("", false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if !strings.Contains(summary, "Config updated") {
		t.Errorf("summary should mention updated config: %s", summary)
	}

	// Verificar que se instalaron skills
	skillsRoot := filepath.Join(tmpDir, "skills")
	if _, err := os.Stat(skillsRoot); os.IsNotExist(err) {
		t.Error("skills dir not created during Sync")
	}

	// Verificar que se creó el symlink
	claudeSkills := filepath.Join(claudeDir, "skills")
	dest, err := os.Readlink(claudeSkills)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if dest != skillsRoot {
		t.Errorf("expected symlink to %s, got %s", skillsRoot, dest)
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
	if _, err := os.Stat(filepath.Join(tmpDir, ".claude", "skills")); os.IsNotExist(err) {
		t.Error("claude skills symlink not created")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".gemini", "skills")); err == nil {
		t.Error("gemini skills symlink should not be created")
	}
}

func TestLLM_LinkSkills_Fallback(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	skillsSource := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsSource, 0755)
	os.WriteFile(filepath.Join(skillsSource, "test.txt"), []byte("test"), 0644)

	llmDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(llmDir, 0755)

	// Simular fallo de symlink creando un archivo donde debería ir el symlink
	// (En realidad, el fallback se dispara si os.Symlink falla, lo cual es difícil
	// de forzar en Linux sin permisos, pero podemos probar copyDir directamente
	// o forzar un error si quisiéramos. Aquí probaremos el comportamiento de limpieza)

	target := filepath.Join(llmDir, "skills")
	os.MkdirAll(target, 0755) // Directorio preexistente (debería ser borrado)

	llm := devflow.NewLLM()
	// No podemos forzar fácilmente el fallo de os.Symlink para probar el fallback de copia
	// sin mockear os.Symlink, pero podemos verificar que linkSkills limpia directorios.

	changed, err := llm.Sync("", false) // Esto usará linkSkills internamente
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if !strings.Contains(changed, "Config updated") {
		t.Errorf("expected config updated, got %s", changed)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("failed to stat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular directory/file (fallback might have triggered or cleanup failed)")
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
