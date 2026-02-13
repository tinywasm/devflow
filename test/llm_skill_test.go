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
	if content == "" {
		t.Fatal("master content is empty")
	}

	// Verificar que contiene marcadores de secciones
	requiredSections := []string{
		"<!-- START_SECTION:CORE_PRINCIPLES -->",
		"<!-- END_SECTION:CORE_PRINCIPLES -->",
		"<!-- START_SECTION:TESTING -->",
		"<!-- START_SECTION:PROTOCOLS -->",
		"<!-- START_SECTION:WASM -->",
		"<!-- START_SECTION:USER_CUSTOM -->",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("master content missing section marker: %s", section)
		}
	}
}

func TestLLM_DetectInstalledLLMs(t *testing.T) {
	// Crear directorio temporal
	tmpDir := t.TempDir()

	// Simular home directory temporal
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
	if err := os.Mkdir(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	installed = llm.DetectInstalledLLMs()
	if len(installed) != 1 {
		t.Fatalf("expected 1 LLM, got %d", len(installed))
	}
	if installed[0].Name != "claude" {
		t.Errorf("expected 'claude', got '%s'", installed[0].Name)
	}

	// Caso 3: Claude y Gemini instalados
	geminiDir := filepath.Join(tmpDir, ".gemini")
	if err := os.Mkdir(geminiDir, 0755); err != nil {
		t.Fatal(err)
	}

	installed = llm.DetectInstalledLLMs()
	if len(installed) != 2 {
		t.Fatalf("expected 2 LLMs, got %d", len(installed))
	}

	names := []string{installed[0].Name, installed[1].Name}
	if !contains(names, "claude") || !contains(names, "gemini") {
		t.Errorf("expected claude and gemini, got %v", names)
	}
}

func TestLLM_SmartSync_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "TEST.md")

	masterContent := `<!-- START_SECTION:TEST -->
Test content
<!-- END_SECTION:TEST -->`

	llm := devflow.NewLLM()
	changed, err := llm.SmartSync(configPath, masterContent)
	if err != nil {
		t.Fatalf("SmartSync failed: %v", err)
	}

	if !changed {
		t.Error("expected changed=true for new file")
	}

	// Verificar que el archivo se creó con el contenido correcto
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}

	if string(content) != masterContent {
		t.Errorf("content mismatch.\nExpected:\n%s\nGot:\n%s", masterContent, string(content))
	}
}

func TestLLM_SmartSync_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "TEST.md")

	masterContent := `<!-- START_SECTION:TEST -->
Test content
<!-- END_SECTION:TEST -->`

	// Crear archivo con contenido idéntico
	if err := os.WriteFile(configPath, []byte(masterContent), 0644); err != nil {
		t.Fatal(err)
	}

	llm := devflow.NewLLM()
	changed, err := llm.SmartSync(configPath, masterContent)
	if err != nil {
		t.Fatalf("SmartSync failed: %v", err)
	}

	if changed {
		t.Error("expected changed=false when content is identical")
	}
}

func TestLLM_SmartSync_UpdateSections(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "TEST.md")

	// Contenido actual con USER_CUSTOM personalizado
	currentContent := `<!-- START_SECTION:CORE -->
Old core content
<!-- END_SECTION:CORE -->

<!-- START_SECTION:USER_CUSTOM -->
My custom rules here
<!-- END_SECTION:USER_CUSTOM -->`

	// Contenido maestro con CORE actualizado
	masterContent := `<!-- START_SECTION:CORE -->
New core content
<!-- END_SECTION:CORE -->

<!-- START_SECTION:USER_CUSTOM -->
<!-- This section is preserved -->
<!-- END_SECTION:USER_CUSTOM -->`

	// Escribir contenido actual
	if err := os.WriteFile(configPath, []byte(currentContent), 0644); err != nil {
		t.Fatal(err)
	}

	llm := devflow.NewLLM()
	changed, err := llm.SmartSync(configPath, masterContent)
	if err != nil {
		t.Fatalf("SmartSync failed: %v", err)
	}

	if !changed {
		t.Error("expected changed=true when section content differs")
	}

	// Leer resultado
	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	resultStr := string(result)

	// Verificar que CORE se actualizó
	if !strings.Contains(resultStr, "New core content") {
		t.Error("CORE section not updated")
	}

	// Verificar que USER_CUSTOM se preservó
	if !strings.Contains(resultStr, "My custom rules here") {
		t.Error("USER_CUSTOM section not preserved")
	}
}

func TestLLM_SmartSync_LegacyFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "TEST.md")

	// Archivo legacy sin marcadores
	legacyContent := `# My Config

Some old content without section markers.`

	masterContent := `<!-- START_SECTION:TEST -->
New sectioned content
<!-- END_SECTION:TEST -->`

	// Escribir contenido legacy
	if err := os.WriteFile(configPath, []byte(legacyContent), 0644); err != nil {
		t.Fatal(err)
	}

	llm := devflow.NewLLM()
	changed, err := llm.SmartSync(configPath, masterContent)
	if err != nil {
		t.Fatalf("SmartSync failed: %v", err)
	}

	if !changed {
		t.Error("expected changed=true for legacy format conversion")
	}

	// Verificar que se creó backup
	backupPath := configPath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("backup file not created for legacy format")
	}

	// Verificar contenido del backup
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupContent) != legacyContent {
		t.Error("backup content doesn't match original")
	}

	// Verificar que el archivo se convirtió
	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != masterContent {
		t.Error("file not converted to master content")
	}
}

func TestLLM_ForceUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "TEST.md")

	existingContent := "Old content"
	masterContent := "New master content"

	// Crear archivo existente
	if err := os.WriteFile(configPath, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	llm := devflow.NewLLM()
	err := llm.ForceUpdate(configPath, masterContent)
	if err != nil {
		t.Fatalf("ForceUpdate failed: %v", err)
	}

	// Verificar backup
	backupPath := configPath + ".bak"
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(backupContent) != existingContent {
		t.Error("backup content incorrect")
	}

	// Verificar sobrescritura
	newContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(newContent) != masterContent {
		t.Error("file not overwritten with master content")
	}
}

func TestLLM_Sync_NoLLMs(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	llm := devflow.NewLLM()
	summary, err := llm.Sync("", false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if summary != "⚠️  No LLMs detected" {
		t.Errorf("expected warning message, got '%s'", summary)
	}
}

func TestLLM_Sync_SpecificLLM(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Crear directorios de LLMs
	claudeDir := filepath.Join(tmpDir, ".claude")
	geminiDir := filepath.Join(tmpDir, ".gemini")
	os.Mkdir(claudeDir, 0755)
	os.Mkdir(geminiDir, 0755)

	llm := devflow.NewLLM()

	// Sync solo Claude
	summary, err := llm.Sync("claude", false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if !strings.Contains(summary, "claude") {
		t.Errorf("summary doesn't mention claude: %s", summary)
	}

	// Verificar que solo Claude se creó
	claudeConfig := filepath.Join(claudeDir, "CLAUDE.md")
	geminiConfig := filepath.Join(geminiDir, "GEMINI.md")

	if _, err := os.Stat(claudeConfig); os.IsNotExist(err) {
		t.Error("Claude config not created")
	}

	if _, err := os.Stat(geminiConfig); err == nil {
		t.Error("Gemini config should not be created")
	}
}

func TestLLM_Sync_NonExistentLLM(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)

	llm := devflow.NewLLM()
	_, err := llm.Sync("copilot", false)
	if err == nil {
		t.Error("expected error for non-existent LLM")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message should mention 'not found', got: %v", err)
	}
}

func TestLLM_Sync_MultipleUpdated(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Crear directorios
	os.Mkdir(filepath.Join(tmpDir, ".claude"), 0755)
	os.Mkdir(filepath.Join(tmpDir, ".gemini"), 0755)

	llm := devflow.NewLLM()
	summary, err := llm.Sync("", false)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Primera ejecución debe actualizar ambos
	if !strings.Contains(summary, "Updated") {
		t.Errorf("expected 'Updated' in summary, got: %s", summary)
	}

	// Segunda ejecución debe skipear ambos
	summary2, err := llm.Sync("", false)
	if err != nil {
		t.Fatalf("Second sync failed: %v", err)
	}

	if !strings.Contains(summary2, "Skipped") {
		t.Errorf("expected 'Skipped' in second summary, got: %s", summary2)
	}
}

func TestExtractSections(t *testing.T) {
	content := `<!-- START_SECTION:SECTION1 -->
Content of section 1
<!-- END_SECTION:SECTION1 -->

<!-- START_SECTION:SECTION2 -->
Content of section 2
Multiple lines here
<!-- END_SECTION:SECTION2 -->`

	sections := devflow.ExtractSections(content)

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}

	expected1 := "Content of section 1"
	if sections["SECTION1"] != expected1 {
		t.Errorf("SECTION1 mismatch.\nExpected: %q\nGot: %q", expected1, sections["SECTION1"])
	}

	expected2 := "Content of section 2\nMultiple lines here"
	if sections["SECTION2"] != expected2 {
		t.Errorf("SECTION2 mismatch.\nExpected: %q\nGot: %q", expected2, sections["SECTION2"])
	}
}

func TestExtractSections_Empty(t *testing.T) {
	content := "No sections here"
	sections := devflow.ExtractSections(content)

	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	content := "Test content"
	if err := os.WriteFile(src, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := devflow.CopyFile(src, dst); err != nil {
		t.Fatalf("devflow.CopyFile failed: %v", err)
	}

	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}

	if string(dstContent) != content {
		t.Errorf("content mismatch.\nExpected: %q\nGot: %q", content, string(dstContent))
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
