package devflow

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const DefaultSkillsReference = "Skills location: ~/tinywasm/skills/"

//go:embed skills
var embeddedSkills embed.FS

// LLMConfig representa la configuración de un LLM específico
type LLMConfig struct {
	Name       string // "claude", "gemini"
	Dir        string // "~/.claude", "~/.gemini"
	ConfigFile string // "CLAUDE.md", "GEMINI.md"
}

// LLM handles synchronization of LLM configuration files and Agent Skills
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

// GetMasterContent retorna la línea de referencia por defecto
func (l *LLM) GetMasterContent() (string, error) {
	return DefaultSkillsReference, nil
}

// InstallSkills instala los Agent Skills embebidos en ~/tinywasm/skills/
func (l *LLM) InstallSkills() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	destRoot := filepath.Join(home, "tinywasm", "skills")
	l.log("Installing skills to:", destRoot)

	return fs.WalkDir(embeddedSkills, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Determinar ruta de destino
		relPath, _ := filepath.Rel("skills", path)
		destPath := filepath.Join(destRoot, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Leer contenido embebido
		data, err := embeddedSkills.ReadFile(path)
		if err != nil {
			return err
		}

		// Escribir archivo (sobrescribir siempre para actualizar)
		return os.WriteFile(destPath, data, 0644)
	})
}

// Sync sincroniza todos los LLMs instalados e instala los skills
func (l *LLM) Sync(specificLLM string, force bool) (string, error) {
	// 1. Instalar skills
	if err := l.InstallSkills(); err != nil {
		return "", fmt.Errorf("failed to install skills: %w", err)
	}

	installed := l.DetectInstalledLLMs()
	if len(installed) == 0 {
		return "⚠️  No LLMs detected (skills installed in ~/tinywasm/skills/)", nil
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
	master = strings.TrimSpace(master)

	var updated []string
	var skipped []string

	for _, llm := range installed {
		configPath := filepath.Join(llm.Dir, llm.ConfigFile)

		changed, err := l.ensureReferenceLine(configPath, master, force)
		if err != nil {
			return "", fmt.Errorf("failed to sync %s: %w", llm.Name, err)
		}

		if changed {
			updated = append(updated, llm.Name)
		} else {
			skipped = append(skipped, llm.Name)
		}
	}

	// Construir resumen
	summary := "Skills updated. "
	if len(updated) > 0 {
		summary += fmt.Sprintf("✅ Config updated: %v", updated)
	}
	if len(skipped) > 0 {
		if len(updated) > 0 {
			summary += ", "
		}
		summary += fmt.Sprintf("⏭️  Config already had reference: %v", skipped)
	}

	return summary, nil
}

// ensureReferenceLine asegura que la línea de referencia existe en el archivo
func (l *LLM) ensureReferenceLine(configPath, line string, force bool) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Si no existe, crearlo
		l.log("Creating new config file:", configPath)
		return true, os.WriteFile(configPath, []byte(line+"\n"), 0644)
	}

	content := string(data)
	if strings.Contains(content, line) && !force {
		return false, nil
	}

	if force {
		// Backup
		backupPath := configPath + ".bak"
		if err := CopyFile(configPath, backupPath); err != nil {
			return false, fmt.Errorf("failed to create backup: %w", err)
		}
		l.log("Created backup:", backupPath)
		
		// En force, podemos decidir si lo re-agregamos al final o qué.
		// Según el plan, solo añadimos la línea si no existe. 
		// Pero si es force, quizá queremos asegurar que esté ahí incluso si el usuario la quitó.
	}

	if !strings.Contains(content, line) {
		l.log("Adding reference line to:", configPath)
		newContent := line + "\n" + content
		return true, os.WriteFile(configPath, []byte(newContent), 0644)
	}

	return false, nil
}

// ForceUpdate reinstala skills y asegura la línea de referencia
func (l *LLM) ForceUpdate(configPath, masterContent string) error {
	if err := l.InstallSkills(); err != nil {
		return err
	}
	_, err := l.ensureReferenceLine(configPath, strings.TrimSpace(masterContent), true)
	return err
}

// CopyFile copia un archivo (helper para backup)
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
