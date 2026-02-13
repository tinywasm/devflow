package devflow_test

import "github.com/tinywasm/devflow"

import (
	"os"
	"strings"
	"testing"
)

func TestUpdateSection(t *testing.T) {
	tmpFile := "test_README.md"
	defer os.Remove(tmpFile)

	// Helper to write file
	writeFile := func(name string, content string) {
		os.WriteFile(name, []byte(content), 0644)
	}

	// Helper to create markdown handler
	newMarkDown := func() *devflow.MarkDown {
		return devflow.NewMarkDown(".", ".", func(name string, data []byte) error {
			return os.WriteFile(name, data, 0644)
		}).InputPath(tmpFile, func(name string) ([]byte, error) {
			return os.ReadFile(name)
		})
	}

	t.Run("CreateNewFile", func(t *testing.T) {
		os.Remove(tmpFile)
		m := newMarkDown()
		content := "New Content"
		err := m.UpdateSection("TEST_SECTION", content)
		if err != nil {
			t.Fatalf("UpdateSection failed: %v", err)
		}

		data, _ := os.ReadFile(tmpFile)
		str := string(data)
		if !strings.Contains(str, content) {
			t.Errorf("Content not found")
		}
		if !strings.Contains(str, "START_SECTION:TEST_SECTION") {
			t.Errorf("Markers not found")
		}
	})

	t.Run("UpdateExistingSection", func(t *testing.T) {
		initial := `# Header
<!-- START_SECTION:TEST -->
Old Content
<!-- END_SECTION:TEST -->
Footer`
		writeFile(tmpFile, initial)

		m := newMarkDown()
		newContent := "New Content"
		err := m.UpdateSection("TEST", newContent)
		if err != nil {
			t.Fatal(err)
		}

		data, _ := os.ReadFile(tmpFile)
		str := string(data)
		if strings.Contains(str, "Old Content") {
			t.Error("Old content persisted")
		}
		if !strings.Contains(str, newContent) {
			t.Error("New content missing")
		}
		if !strings.Contains(str, "# Header") || !strings.Contains(str, "Footer") {
			t.Error("Surrounding content lost")
		}
	})

	t.Run("NoChangeIdentical", func(t *testing.T) {
		content := "Same Content"
		initial := fmtSection("TEST", content)
		writeFile(tmpFile, initial)

		statBefore, _ := os.Stat(tmpFile)

		m := newMarkDown()
		err := m.UpdateSection("TEST", content)
		if err != nil {
			t.Fatal(err)
		}

		statAfter, _ := os.Stat(tmpFile)
		if !statAfter.ModTime().Equal(statBefore.ModTime()) {
			t.Error("File modified when content identical")
		}
	})

	t.Run("PositionAfterLine", func(t *testing.T) {
		initial := "Line 1\nLine 2\nLine 3"
		writeFile(tmpFile, initial)

		m := newMarkDown()
		// Insert after line 1
		err := m.UpdateSection("TEST", "Content", "1")
		if err != nil {
			t.Fatal(err)
		}

		data, _ := os.ReadFile(tmpFile)
		lines := strings.Split(string(data), "\n")
		// Line 1 is index 0. "After line 1" means insert at index 1?
		// Logic: 1-based "1" -> 0-based index 1.
		// [0] Line 1. [1] Section... [2] Line 2.
		if lines[0] != "Line 1" {
			t.Errorf("Line 1 moved: %s", lines[0])
		}
		if !strings.Contains(lines[1], "START_SECTION:TEST") {
			t.Errorf("Section not at line 2: %s", lines[1])
		}
	})

	t.Run("DeduplicateSections", func(t *testing.T) {
		initial := `
<!-- START_SECTION:DUP -->
one
<!-- END_SECTION:DUP -->
<!-- START_SECTION:DUP -->
two
<!-- END_SECTION:DUP -->
`
		writeFile(tmpFile, initial)

		m := newMarkDown()
		err := m.UpdateSection("DUP", "Fixed")
		if err != nil {
			t.Fatal(err)
		}

		data, _ := os.ReadFile(tmpFile)
		str := string(data)
		if strings.Count(str, "START_SECTION:DUP") != 1 {
			t.Error("Duplicates not removed")
		}
		if !strings.Contains(str, "Fixed") {
			t.Error("Content not updated")
		}
	})

	t.Run("RelocateMisplacedSection", func(t *testing.T) {
		// Section exists at bottom but should be after line 1 (title)
		initial := `# Title
Some content
Footer
<!-- START_SECTION:BADGES_SECTION -->
Old badges
<!-- END_SECTION:BADGES_SECTION -->`
		writeFile(tmpFile, initial)

		m := newMarkDown()
		// afterLine="1" means insert after line 1 (the title)
		err := m.UpdateSection("BADGES_SECTION", "New badges", "1")
		if err != nil {
			t.Fatal(err)
		}

		data, _ := os.ReadFile(tmpFile)
		lines := strings.Split(string(data), "\n")

		// Line 0 should be "# Title"
		if lines[0] != "# Title" {
			t.Errorf("Title moved: got %q", lines[0])
		}
		// Line 1 should be the section start (after title)
		if !strings.Contains(lines[1], "START_SECTION:BADGES_SECTION") {
			t.Errorf("Section not relocated to after title: line 1 = %q", lines[1])
		}
		// Verify old position is empty (section was moved, not duplicated)
		str := string(data)
		if strings.Count(str, "START_SECTION:BADGES_SECTION") != 1 {
			t.Error("Section duplicated instead of relocated")
		}
		if !strings.Contains(str, "New badges") {
			t.Error("Content not updated")
		}
	})
}

func fmtSection(id, content string) string {
	return "<!-- START_SECTION:" + id + " -->\n" + content + "\n<!-- END_SECTION:" + id + " -->"
}
