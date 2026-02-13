package devflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Bashrc handles updates to .bashrc file using markers
type Bashrc struct {
	FilePath string
}

// NewBashrc creates a new Bashrc handler for ~/.bashrc
func NewBashrc() *Bashrc {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return &Bashrc{
		FilePath: filepath.Join(home, ".bashrc"),
	}
}

// Set updates or creates a variable in .bashrc
// If value is empty, removes the variable
func (b *Bashrc) Set(key, value string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// If value is empty, remove the variable
	if value == "" {
		return b.remove(key)
	}

	sectionID := key
	// Escape internal quotes for proper bash syntax
	escapedValue := strings.ReplaceAll(value, `"`, `\"`)
	content := fmt.Sprintf("export %s=\"%s\"", key, escapedValue)

	return b.updateSection(sectionID, content)
}

// Get reads a variable value from .bashrc file
func (b *Bashrc) Get(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key cannot be empty")
	}

	content, err := b.readFile()
	if err != nil {
		return "", err
	}

	startMarker := fmt.Sprintf("# START_DEVFLOW:%s", key)
	endMarker := fmt.Sprintf("# END_DEVFLOW:%s", key)

	sections, err := b.findAllSections(content, startMarker, endMarker)
	if err != nil {
		return "", err
	}

	if len(sections) == 0 {
		return "", fmt.Errorf("variable %s not found in .bashrc", key)
	}

	// Extract value from export statement
	// Content format: export KEY="value"
	exportLine := sections[0].content
	return b.ExtractValue(exportLine, key)
}

// updateSection updates or creates a section in .bashrc
func (b *Bashrc) updateSection(sectionID, content string) error {
	startMarker := fmt.Sprintf("# START_DEVFLOW:%s", sectionID)
	endMarker := fmt.Sprintf("# END_DEVFLOW:%s", sectionID)

	newSection := fmt.Sprintf("%s\n%s\n%s", startMarker, content, endMarker)

	currentContent, err := b.readFile()
	if err != nil {
		// If file doesn't exist, create with new section
		if os.IsNotExist(err) {
			return b.writeFile(newSection + "\n")
		}
		return err
	}

	// Find all sections with this ID
	sections, err := b.findAllSections(currentContent, startMarker, endMarker)
	if err != nil {
		return err
	}

	// Check if content is the same (no update needed)
	if len(sections) == 1 && sections[0].content == content {
		return nil // No change needed
	}

	// Remove all existing sections
	contentWithoutSections := b.removeAllSections(currentContent, sections)

	// Append new section at end
	newContent := strings.TrimSpace(contentWithoutSections) + "\n" + newSection + "\n"

	return b.writeFile(newContent)
}

// remove deletes a variable section from .bashrc
func (b *Bashrc) remove(key string) error {
	content, err := b.readFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return err
	}

	startMarker := fmt.Sprintf("# START_DEVFLOW:%s", key)
	endMarker := fmt.Sprintf("# END_DEVFLOW:%s", key)

	sections, err := b.findAllSections(content, startMarker, endMarker)
	if err != nil {
		return err
	}

	if len(sections) == 0 {
		return nil // Nothing to remove
	}

	// Remove all sections
	newContent := b.removeAllSections(content, sections)
	return b.writeFile(newContent)
}

// readFile reads .bashrc content
func (b *Bashrc) readFile() (string, error) {
	data, err := os.ReadFile(b.FilePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeFile writes content to .bashrc
func (b *Bashrc) writeFile(content string) error {
	return os.WriteFile(b.FilePath, []byte(content), 0644)
}

type sectionInfo struct {
	startLine int
	endLine   int
	content   string
}

// findAllSections finds all sections with given markers
func (b *Bashrc) findAllSections(content, startMarker, endMarker string) ([]sectionInfo, error) {
	lines := strings.Split(content, "\n")
	var sections []sectionInfo
	currentStart := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == startMarker {
			currentStart = i
		} else if trimmed == endMarker && currentStart >= 0 {
			// Extract content between markers
			var sectionContent strings.Builder
			for j := currentStart + 1; j < i; j++ {
				if j > currentStart+1 {
					sectionContent.WriteString("\n")
				}
				sectionContent.WriteString(lines[j])
			}

			sections = append(sections, sectionInfo{
				startLine: currentStart,
				endLine:   i,
				content:   sectionContent.String(),
			})
			currentStart = -1
		}
	}

	return sections, nil
}

// removeAllSections removes all sections from content
func (b *Bashrc) removeAllSections(content string, sections []sectionInfo) string {
	if len(sections) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")

	// Remove from end to start to preserve indices
	for i := len(sections) - 1; i >= 0; i-- {
		section := sections[i]
		var newLines []string
		if section.startLine > 0 {
			newLines = append(newLines, lines[:section.startLine]...)
		}
		if section.endLine+1 < len(lines) {
			newLines = append(newLines, lines[section.endLine+1:]...)
		}
		lines = newLines
	}

	return strings.Join(lines, "\n")
}

// ExtractValue extracts value from export statement
// Input: export KEY="value" or export KEY=value
// Output: value
func (b *Bashrc) ExtractValue(exportLine, key string) (string, error) {
	// Remove leading/trailing whitespace
	line := strings.TrimSpace(exportLine)

	// Expected format: export KEY="value"
	prefix := "export " + key + "="
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("invalid export format: %s", line)
	}

	// Extract value part
	value := strings.TrimPrefix(line, prefix)

	// Remove outer quotes and unescape internal quotes
	value = strings.Trim(value, "\"")
	value = strings.ReplaceAll(value, `\"`, `"`)

	return value, nil
}
