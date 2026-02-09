package devflow

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed DEFAULT_GLOBAL_LLM_SKILLS.md
var defaultLLMSkills embed.FS

// LLMConfig representa la configuración de un LLM específico
type LLMConfig struct {
	Name       string // "claude", "gemini"
	Dir        string // "~/.claude", "~/.gemini"
	ConfigFile string // "CLAUDE.md", "GEMINI.md"
}

// LLM handles synchronization of LLM configuration files
type LLM struct {
	log func(...any)
}

// NewLLM creates a new LLM handler
func NewLLM() *LLM {
	return &LLM{
		log: func(...any) {},
	}
}

// SetLog sets the logger function
func (l *LLM) SetLog(fn func(...any)) {
	if fn != nil {
		l.log = fn
	}
}

// GetSupportedLLMs retorna la lista de LLMs soportados
func (l *LLM) GetSupportedLLMs() []LLMConfig {
	home, _ := os.UserHomeDir()
	return []LLMConfig{
		{Name: "claude", Dir: filepath.Join(home, ".claude"), ConfigFile: "CLAUDE.md"},
		{Name: "gemini", Dir: filepath.Join(home, ".gemini"), ConfigFile: "GEMINI.md"},
	}
}

// DetectInstalledLLMs detecta qué LLMs están instalados
// Returns: lista de LLMConfig para los LLMs instalados
func (l *LLM) DetectInstalledLLMs() []LLMConfig {
	var installed []LLMConfig
	for _, llm := range l.GetSupportedLLMs() {
		if _, err := os.Stat(llm.Dir); err == nil {
			installed = append(installed, llm)
			l.log("Detected LLM:", llm.Name, "at", llm.Dir)
		}
	}
	return installed
}

// GetMasterContent lee el contenido del archivo maestro embebido
func (l *LLM) GetMasterContent() (string, error) {
	content, err := defaultLLMSkills.ReadFile("DEFAULT_GLOBAL_LLM_SKILLS.md")
	if err != nil {
		return "", fmt.Errorf("failed to read master template: %w", err)
	}
	return string(content), nil
}

// Sync sincroniza todos los LLMs instalados
// Returns: resumen de operaciones realizadas
func (l *LLM) Sync(specificLLM string, force bool) (string, error) {
	installed := l.DetectInstalledLLMs()

	if len(installed) == 0 {
		return "⚠️  No LLMs detected", nil
	}

	// Filtrar por LLM específico si se proporcionó
	if specificLLM != "" {
		var filtered []LLMConfig
		for _, llm := range installed {
			if llm.Name == specificLLM {
				filtered = append(filtered, llm)
				break
			}
		}
		if len(filtered) == 0 {
			return "", fmt.Errorf("LLM '%s' not found or not installed", specificLLM)
		}
		installed = filtered
	}

	master, err := l.GetMasterContent()
	if err != nil {
		return "", err
	}

	var updated []string
	var skipped []string

	for _, llm := range installed {
		configPath := filepath.Join(llm.Dir, llm.ConfigFile)

		if force {
			if err := l.forceUpdate(configPath, master); err != nil {
				return "", fmt.Errorf("failed to update %s: %w", llm.Name, err)
			}
			updated = append(updated, llm.Name)
		} else {
			changed, err := l.smartSync(configPath, master)
			if err != nil {
				return "", fmt.Errorf("failed to sync %s: %w", llm.Name, err)
			}
			if changed {
				updated = append(updated, llm.Name)
			} else {
				skipped = append(skipped, llm.Name)
			}
		}
	}

	// Construir resumen
	summary := ""
	if len(updated) > 0 {
		summary += fmt.Sprintf("✅ Updated: %v", updated)
	}
	if len(skipped) > 0 {
		if summary != "" {
			summary += ", "
		}
		summary += fmt.Sprintf("⏭️  Skipped (up-to-date): %v", skipped)
	}

	return summary, nil
}

// smartSync realiza sincronización inteligente con merge de marcadores
func (l *LLM) smartSync(configPath, masterContent string) (bool, error) {
	// Leer contenido actual
	currentContent, err := os.ReadFile(configPath)
	hasExisting := err == nil

	if !hasExisting {
		// Archivo no existe, crear nuevo
		l.log("Creating new config file:", configPath)
		if err := os.WriteFile(configPath, []byte(masterContent), 0644); err != nil {
			return false, err
		}
		return true, nil
	}

	current := string(currentContent)

	// Si el contenido es idéntico, skip
	if current == masterContent {
		l.log("Config already up-to-date:", configPath)
		return false, nil
	}

	// Extraer secciones del master y del archivo actual
	masterSections := extractSections(masterContent)
	currentSections := extractSections(current)

	// Si el archivo actual no tiene secciones (formato legacy), hacer backup y reemplazar
	if len(currentSections) == 0 {
		l.log("Legacy format detected, converting to sectioned format:", configPath)
		backupPath := configPath + ".bak"
		if err := copyFile(configPath, backupPath); err != nil {
			return false, fmt.Errorf("failed to create backup: %w", err)
		}
		l.log("Created backup:", backupPath)
		if err := os.WriteFile(configPath, []byte(masterContent), 0644); err != nil {
			return false, err
		}
		return true, nil
	}

	// Usar MarkDown.UpdateSection para actualizar secciones
	md := NewMarkDown(filepath.Dir(configPath), filepath.Dir(configPath),
		func(name string, data []byte) error {
			return os.WriteFile(name, data, 0644)
		})
	md.InputPath(configPath, os.ReadFile)
	md.SetLog(l.log)

	changed := false
	for sectionID, content := range masterSections {
		// Skip USER_CUSTOM ya que es del usuario (no sobrescribir su contenido)
		if sectionID == "USER_CUSTOM" {
			// Pero si no existe en el archivo actual, agregarla como placeholder
			if _, exists := currentSections["USER_CUSTOM"]; !exists {
				if err := md.UpdateSection("USER_CUSTOM", content); err != nil {
					return false, fmt.Errorf("failed to add USER_CUSTOM section: %w", err)
				}
				changed = true
			}
			continue
		}

		// Solo actualizar si el contenido de la sección cambió
		if currentContent, exists := currentSections[sectionID]; !exists || currentContent != content {
			// Actualizar sección
			if err := md.UpdateSection(sectionID, content); err != nil {
				return false, fmt.Errorf("failed to update section %s: %w", sectionID, err)
			}
			changed = true
		}
	}

	return changed, nil
}

// forceUpdate sobrescribe completamente el archivo (con backup)
func (l *LLM) forceUpdate(configPath, masterContent string) error {
	// Crear backup si existe
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".bak"
		if err := copyFile(configPath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		l.log("Created backup:", backupPath)
	}

	// Sobrescribir
	if err := os.WriteFile(configPath, []byte(masterContent), 0644); err != nil {
		return err
	}

	return nil
}

// extractSections extrae secciones marcadas del contenido
func extractSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentID string
	var currentContent []string
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detectar inicio de sección
		if strings.HasPrefix(trimmed, "<!-- START_SECTION:") {
			currentID = strings.TrimSuffix(
				strings.TrimPrefix(trimmed, "<!-- START_SECTION:"),
				" -->",
			)
			currentContent = []string{}
			inSection = true
			continue
		}

		// Detectar fin de sección
		if strings.HasPrefix(trimmed, "<!-- END_SECTION:") {
			if inSection {
				sections[currentID] = strings.Join(currentContent, "\n")
				inSection = false
			}
			continue
		}

		// Acumular contenido
		if inSection {
			currentContent = append(currentContent, line)
		}
	}

	return sections
}

// copyFile copia un archivo (helper para backup)
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
