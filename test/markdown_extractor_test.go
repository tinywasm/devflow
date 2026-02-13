package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writerWithDirCreation returns a writer function that creates directories before writing
func writerWithDirCreation() func(name string, data []byte) error {
	return func(name string, data []byte) error {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			return err
		}
		return os.WriteFile(name, data, 0644)
	}
}

func TestExtractCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "templates")
	outputDir := filepath.Join(tmp, "output")

	// Create source directory and markdown file
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("creating source dir: %v", err)
	}

	// Create a test markdown file with Go code
	mdContent := `# Test Server
This is a test markdown file.

` + "```go" + `
package main

import "flag"

func main() {
	port := flag.String("port", "9090", "server port")
	publicDir := flag.String("public-dir", "public", "public directory")
	flag.Parse()
	
	if *publicDir == "" {
		*publicDir = "public"
	}
}
` + "```" + `

More text here.

` + "```go" + `
func noCache() {
	// middleware function
}
` + "```"

	mdFile := filepath.Join(sourceDir, "server.md")
	if err := os.WriteFile(mdFile, []byte(mdContent), 0644); err != nil {
		t.Fatalf("writing test markdown: %v", err)
	}

	// Create MarkDown instance (provide writer and reader functions)
	m := devflow.NewMarkDown(tmp, outputDir, writerWithDirCreation()).
		InputPath("templates/server.md", func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(tmp, name)) })

	// Ensure output file doesn't exist yet
	outputFile := filepath.Join(outputDir, "main.go")
	if _, err := os.Stat(outputFile); err == nil {
		t.Fatalf("expected no existing file at %s", outputFile)
	}

	// Extract Go code
	if err := m.Extract("main.go"); err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Read generated file
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	contentStr := string(content)

	// Verify content
	if !strings.Contains(contentStr, "package main") {
		t.Errorf("generated file missing package main")
	}
	if !strings.Contains(contentStr, "9090") {
		t.Errorf("generated file missing port 9090")
	}
	if !strings.Contains(contentStr, `flag.String`) {
		t.Errorf("generated file missing flag.String usage")
	}
	if !strings.Contains(contentStr, `public-dir`) {
		t.Errorf("generated file missing public-dir flag")
	}
	if !strings.Contains(contentStr, `noCache`) {
		t.Errorf("generated file missing noCache function")
	}
}

func TestExtractDoesNotOverwriteIfSame(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "templates")
	outputDir := filepath.Join(tmp, "output")

	// Create directories
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("creating source dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("creating output dir: %v", err)
	}

	// Create markdown with code
	mdContent := "```go\npackage main\n```"
	mdFile := filepath.Join(sourceDir, "test.md")
	if err := os.WriteFile(mdFile, []byte(mdContent), 0644); err != nil {
		t.Fatalf("writing markdown: %v", err)
	}

	// Create MarkDown instance (provide writer and reader functions)
	m := devflow.NewMarkDown(tmp, outputDir, func(name string, data []byte) error { return os.WriteFile(name, data, 0644) }).
		InputPath("templates/test.md", func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(tmp, name)) })

	// First extraction
	outputFile := "test.go"
	if err := m.Extract(outputFile); err != nil {
		t.Fatalf("first extract failed: %v", err)
	}

	// Get file info
	outputPath := filepath.Join(outputDir, outputFile)
	info1, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat after first extract: %v", err)
	}

	// Second extraction (should not overwrite since content is the same)
	if err := m.Extract(outputFile); err != nil {
		t.Fatalf("second extract failed: %v", err)
	}

	// Check modification time hasn't changed (file wasn't rewritten)
	info2, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat after second extract: %v", err)
	}

	if info1.ModTime() != info2.ModTime() {
		t.Errorf("file was rewritten when it shouldn't have been")
	}
}

func TestExtractOverwritesIfDifferent(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "templates")
	outputDir := filepath.Join(tmp, "output")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("creating source dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("creating output dir: %v", err)
	}

	// Create output file with different content
	outputPath := filepath.Join(outputDir, "test.go")
	originalContent := "package original"
	if err := os.WriteFile(outputPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("writing original file: %v", err)
	}

	// Create markdown with different code
	mdContent := "```go\npackage updated\n```"
	mdFile := filepath.Join(sourceDir, "test.md")
	if err := os.WriteFile(mdFile, []byte(mdContent), 0644); err != nil {
		t.Fatalf("writing markdown: %v", err)
	}

	// Extract (should overwrite)
	m := devflow.NewMarkDown(tmp, outputDir, func(name string, data []byte) error { return os.WriteFile(name, data, 0644) }).
		InputPath("templates/test.md", func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(tmp, name)) })
	if err := m.Extract("test.go"); err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Verify content was updated
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	if string(content) == originalContent {
		t.Errorf("file was not overwritten when content was different")
	}
	if !strings.Contains(string(content), "package updated") {
		t.Errorf("file doesn't contain new content")
	}
}

func TestExtractConcatenatesMultipleBlocks(t *testing.T) {
	tmp := t.TempDir()

	// Markdown with multiple Go blocks
	md := "Some text\n```go\npackage main\n\nfunc A(){}\n```\nMore\n```go\nfunc B(){}\n```\n"

	m := devflow.NewMarkDown(tmp, tmp, func(name string, data []byte) error { return os.WriteFile(name, data, 0644) }).
		InputByte([]byte(md))

	// Extract using byte slice
	if err := m.Extract("output.go"); err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Read output
	content, err := os.ReadFile(filepath.Join(tmp, "output.go"))
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "func A()") || !strings.Contains(contentStr, "func B()") {
		t.Fatalf("extraction failed, got: %s", contentStr)
	}

	// Ensure both blocks are present
	if strings.Count(contentStr, "func") < 2 {
		t.Fatalf("expected both functions present, got: %s", contentStr)
	}
}

func TestExtractWithByteSlice(t *testing.T) {
	tmp := t.TempDir()
	m := devflow.NewMarkDown(tmp, tmp, func(name string, data []byte) error { return os.WriteFile(name, data, 0644) }).
		InputByte([]byte("# Test\n```go\npackage test\n```"))

	if err := m.Extract("test.go"); err != nil {
		t.Fatalf("extract with byte slice failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmp, "test.go"))
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}

	if !strings.Contains(string(content), "package test") {
		t.Errorf("extracted content missing expected code")
	}
}
