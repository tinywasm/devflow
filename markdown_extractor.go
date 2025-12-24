package devflow

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Extract extracts code blocks from the configured input and writes to outputFile
// The output file extension determines which code type to extract (.go, .js, .css)
func (m *MarkDown) Extract(outputFile string) error {
	if m.destination == "" {
		return fmt.Errorf("destination not set; provide destination when calling NewMarkDown(rootDir, destination)")
	}

	// Read markdown from the configured input
	markdown, err := m.readFile(m.inputPath)
	if err != nil {
		return fmt.Errorf("reading file %s: %v", m.inputPath, err)
	}

	// Determine code type from output file extension
	codeType := m.getCodeType(outputFile)
	if codeType == "" {
		return fmt.Errorf("unsupported file extension: %s", filepath.Ext(outputFile))
	}

	// Extract code blocks
	code := m.extractCodeBlocks(string(markdown), codeType)
	if code == "" {
		return fmt.Errorf("no %s code blocks found in markdown", codeType)
	}

	// Write to output file
	outputPath := filepath.Join(m.destination, outputFile)
	if err := m.writeIfDifferent(outputPath, code); err != nil {
		return fmt.Errorf("writing output file: %v", err)
	}

	m.log("Extracted", codeType, "code to", outputPath)

	return nil
}

// getCodeType determines the code type from file extension
func (m *MarkDown) getCodeType(outputFile string) string {
	ext := filepath.Ext(outputFile)
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".css":
		return "css"
	default:
		return ""
	}
}

// extractCodeBlocks extracts code blocks of a specific type from markdown content
func (m *MarkDown) extractCodeBlocks(markdown, codeType string) string {
	var result strings.Builder
	lines := strings.Split(markdown, "\n")
	inBlock := false

	// Format of code block start: ```codeType or ``` codeType
	codeBlockStart := "```" + codeType

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			// Check if it's start or end
			if !inBlock {
				// Check if it's the start of our code type
				if strings.HasPrefix(trimmed, codeBlockStart) {
					inBlock = true
				}
			} else {
				// It's the end of a block
				// But we need to be careful if it's nested (unlikely in this simple parser)
				// Assuming standard markdown where blocks are not nested
				if trimmed == "```" {
					inBlock = false
					result.WriteString("\n") // Add separation between blocks
				}
			}
		} else if inBlock {
			result.WriteString(line + "\n")
		}
	}

	return strings.TrimSpace(result.String())
}
