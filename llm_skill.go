package devflow

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

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

// InstallSkills instala los Agent Skills embebidos en ~/skills/
func (l *LLM) InstallSkills() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	destRoot := filepath.Join(home, "skills")
	l.log("Installing skills to:", destRoot)

	err = fs.WalkDir(embeddedSkills, "skills", func(path string, d fs.DirEntry, err error) error {
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
	return destRoot, err
}

// Sync sincroniza todos los LLMs instalados e instala los skills
func (l *LLM) Sync(specificLLM string, force bool) (string, error) {
	// 1. Instalar skills
	destRoot, err := l.InstallSkills()
	if err != nil {
		return "", fmt.Errorf("failed to install skills: %w", err)
	}

	installed := l.DetectInstalledLLMs()
	if len(installed) == 0 {
		return "⚠️  No LLMs detected (skills installed in ~/skills/)", nil
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

	var updated []string
	var skipped []string

	for _, llm := range installed {
		changed, err := l.linkSkills(llm.Dir, destRoot)
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

// CopyFile copia un archivo (helper para backup)
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// linkSkills creates a symlink from the LLM's skills dir to the shared skills location.
// Falls back to copying if symlink fails (Windows without Developer Mode).
func (l *LLM) linkSkills(llmDir, skillsSource string) (bool, error) {
	target := filepath.Join(llmDir, "skills")

	// Already correct symlink?
	if dest, err := os.Readlink(target); err == nil {
		if dest == skillsSource {
			return false, nil // already linked
		}
		os.Remove(target) // stale symlink
	}

	// Remove if regular dir exists (leftover from old copy approach)
	if info, err := os.Lstat(target); err == nil && info.IsDir() {
		os.RemoveAll(target)
	}

	// Try symlink
	if err := os.Symlink(skillsSource, target); err == nil {
		return true, nil
	}

	// Fallback: copy only our own skills (not the whole dir)
	return true, copyDir(skillsSource, target)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
