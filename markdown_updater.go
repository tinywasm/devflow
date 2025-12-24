package devflow

import (
	"fmt"
	"strconv"
	"strings"
)

// UpdateSection updates or creates a section in the input file based on identifier.
// sectionID: The identifier for the section (e.g., "BADGES").
// content: The new content to insert.
// afterLine: Optional. Implementation tries to insert after this line number if section doesn't exist.
func (m *MarkDown) UpdateSection(sectionID, content string, afterLine ...string) error {
	if sectionID == "" || content == "" {
		return fmt.Errorf("section_identifier and new_content are required")
	}

	// Handle special case for BADGES -> BADGES_SECTION for backward compatibility
	if sectionID == "BADGES" {
		sectionID = "BADGES_SECTION"
	}

	// Read from configured input
	existing, err := m.readFile(m.inputPath)

	currentContent := ""
	if err == nil {
		currentContent = string(existing)
	}

	// Logic from devscripts
	newContent, changed, err := m.processContent(currentContent, sectionID, content, afterLine...)
	if err != nil {
		return err
	}

	if !changed {
		m.log("Section", sectionID, "already up to date")
		return nil
	}

	if err := m.writeFile(m.inputPath, []byte(newContent)); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	m.log("Updated section", sectionID, "successfully")
	return nil
}

func (m *MarkDown) processContent(currentContent, sectionID, content string, afterLineArgs ...string) (string, bool, error) {
	sectionStart := fmt.Sprintf("<!-- START_SECTION:%s -->", sectionID)
	sectionEnd := fmt.Sprintf("<!-- END_SECTION:%s -->", sectionID)

	// Create new section content
	newSection := fmt.Sprintf("%s\n%s\n%s", sectionStart, content, sectionEnd)

	// If file is empty or doesn't exist (currentContent empty usually implies new)
	if currentContent == "" {
		return newSection + "\n", true, nil
	}

	// Find all duplicate sections and their positions
	sections, err := m.findAllSections(currentContent, sectionStart, sectionEnd)
	if err != nil {
		return "", false, err
	}

	// Determine insertion position
	afterLine := ""
	if len(afterLineArgs) > 0 {
		afterLine = afterLineArgs[0]
	}
	insertPos := m.determineInsertPosition(currentContent, sections, afterLine)

	// Check if content needs updating
	if len(sections) == 1 && sections[0].content == content {
		// Content is the same, check position
		if afterLine == "" || sections[0].startLine == insertPos {
			return currentContent, false, nil // No change needed
		}
	}

	// Remove all existing sections
	contentWithoutSections := m.removeAllSections(currentContent, sections)

	// Insert new section at determined position
	newContent := m.insertSectionAtPosition(contentWithoutSections, newSection, insertPos)

	return newContent, true, nil
}

func (m *MarkDown) findAllSections(content, startMarker, endMarker string) ([]sectionInfo, error) {
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

func (m *MarkDown) determineInsertPosition(content string, sections []sectionInfo, afterLine string) int {
	lines := strings.Split(content, "\n")

	if afterLine != "" {
		// Parse after_line parameter (1-based from user input)
		if pos, err := strconv.Atoi(afterLine); err == nil {
			if pos >= 1 && pos <= len(lines) {
				return pos // Convert to 0-based: after line 1 means position 1 (slice index)
			}
		}
	}

	// Default behavior: if sections exist, use first section position
	if len(sections) > 0 {
		return sections[0].startLine
	}

	// No sections exist and no after_line specified, append at end
	return len(lines)
}

func (m *MarkDown) removeAllSections(content string, sections []sectionInfo) string {
	if len(sections) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")

	// Sort sections by start line in descending order to remove from end
	// Note: They should appear in order anyway if scan was linear, but safe to sort?
	// But let's reverse iterate
	for i := len(sections) - 1; i >= 0; i-- {
		section := sections[i]
		// Remove lines from endLine to startLine (inclusive)

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

func (m *MarkDown) insertSectionAtPosition(content, section string, position int) string {
	lines := strings.Split(content, "\n")

	// Ensure position is within bounds
	if position > len(lines) {
		position = len(lines)
	}
	if position < 0 {
		position = 0
	}

	// Insert section at position
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:position]...)
	newLines = append(newLines, section)
	if position < len(lines) {
		newLines = append(newLines, lines[position:]...)
	}

	return strings.Join(newLines, "\n")
}
