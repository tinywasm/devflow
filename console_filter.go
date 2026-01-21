package devflow

import (
	"fmt"
	"strings"
)

type ConsoleFilter struct {
	buffer         []string
	output         func(string) // callback to write output
	hasDataRace    bool
	shownRaceMsg   bool
	incompleteLine string
}

func NewConsoleFilter(output func(string)) *ConsoleFilter {
	if output == nil {
		output = func(s string) { fmt.Println(s) }
	}
	return &ConsoleFilter{
		output: output,
	}
}

func (cf *ConsoleFilter) Add(input string) {
	// Handle fragmentation: ensure we only process complete lines (ending in \n)
	fullInput := cf.incompleteLine + input
	lines := strings.Split(fullInput, "\n")

	// The last element of Split is either an empty string (if input ended in \n)
	// or the incomplete part of the line.
	cf.incompleteLine = lines[len(lines)-1]

	// Process all complete lines
	for i := 0; i < len(lines)-1; i++ {
		cf.addLine(lines[i])
	}
}

func (cf *ConsoleFilter) addLine(line string) {
	// ALWAYS show DEBUG messages
	if strings.Contains(line, "DEBUG") {
		cf.output(line)
		return
	}

	// Detect data races
	if strings.Contains(line, "WARNING: DATA RACE") {
		cf.hasDataRace = true
		return // Skip individual warnings
	}

	// Skip packages with no test files (noise)
	if strings.Contains(line, "[no test files]") {
		return
	}

	// Skip noise
	if strings.HasPrefix(line, "go: warning:") ||
		strings.HasPrefix(line, "#") ||
		strings.HasPrefix(line, "package ") ||
		strings.HasPrefix(line, "ok\t") ||
		strings.HasPrefix(line, "ok  \t") ||
		strings.Contains(line, "build constraints exclude all Go files") ||
		strings.Contains(line, "[setup failed]") ||
		strings.Contains(line, "no packages to test") ||
		strings.Contains(line, "(cached)") ||
		strings.Contains(line, "✅ All tests passed!") ||
		strings.Contains(line, "All tests passed!") ||
		strings.Contains(line, "❌ WASM tests failed") ||
		strings.Contains(line, "WASM tests failed") ||
		strings.Contains(line, "Badges saved to") ||
		strings.Contains(line, "tests passed") ||
		(strings.Contains(line, "coverage:") && strings.Contains(line, "% of statements")) ||
		line == "FAIL" ||
		line == "PASS" ||
		strings.HasPrefix(line, "exit with status") ||
		strings.HasPrefix(line, "exit status") ||
		// Data race details
		strings.HasPrefix(line, "Read at ") ||
		strings.HasPrefix(line, "Write at ") ||
		strings.HasPrefix(line, "Previous write at ") ||
		strings.HasPrefix(line, "Previous read at ") ||
		strings.Contains(line, "by goroutine") ||
		// Panic/crash details
		strings.HasPrefix(line, "[signal ") ||
		strings.HasPrefix(line, "goroutine ") ||
		strings.HasPrefix(line, "created by ") {
		return
	}

	trimmed := strings.TrimSpace(line)

	// Skip stack traces from stdlib (/usr/local/go, /usr/lib/go)
	if strings.HasPrefix(trimmed, "/usr/") {
		return
	}

	// Keep first project file reference, skip subsequent ones
	// Format: /path/to/project/file.go:line +0xhex
	if strings.HasPrefix(trimmed, "/") && strings.Contains(trimmed, ".go:") {
		// Extract just filename:line from full path
		parts := strings.Split(trimmed, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Remove hex offset like +0x38
			if idx := strings.Index(lastPart, " +0x"); idx != -1 {
				lastPart = lastPart[:idx]
			}
			// Add shortened reference and continue filtering
			cf.buffer = append(cf.buffer, "    "+lastPart)
		}
		return
	}

	// NOTE: We no longer filter indented .go: lines here because they may contain
	// test error messages. The removePassingTestLogs mechanism handles cleanup
	// for passing tests, so these lines will only remain for failing tests.

	// Skip function calls with memory addresses like TestNilPointer(0xc0000a6b60)
	if strings.Contains(line, "(0x") && !strings.Contains(line, ".go:") {
		return
	}

	// Global markers - flush buffer and skip the marker itself
	if strings.HasPrefix(line, "FAIL\t") ||
		strings.HasPrefix(line, "ok\t") ||
		strings.HasPrefix(line, "coverage:") ||
		strings.HasPrefix(line, "pkg:") {
		cf.Flush()
		return
	}

	// Keep error lines
	cf.buffer = append(cf.buffer, line)

	// Remove passing test logs
	if strings.Contains(line, "--- PASS:") {
		cf.removePassingTestLogs(line)
	}
}

func (cf *ConsoleFilter) removePassingTestLogs(passLine string) {
	// Extract TestName from passLine
	// Fields: "---", "PASS:", "TestName", "(0.00s)"
	fields := strings.Fields(passLine)
	var testName string
	for i, f := range fields {
		if f == "PASS:" && i+1 < len(fields) {
			testName = fields[i+1]
			break
		}
	}

	if testName == "" {
		return
	}

	// Search backwards for "=== RUN TestName"
	foundRun := -1
	runLinesInBetween := false

	// Iterate backwards from the line before the PASS line
	// (PASS line is already in buffer at last index)
	searchStart := len(cf.buffer) - 2
	/*
		if searchStart < 0 {
			// Even if we can't search backwards, we should fall through to the "RUN not found" logic
			// which removes the PASS line itself. This handles "orphaned" PASS lines where the RUN
			// line was flushed earlier or missing.
		}
	*/

	for i := searchStart; i >= 0; i-- {
		lineFields := strings.Fields(cf.buffer[i])
		if len(lineFields) >= 3 && lineFields[0] == "===" && lineFields[1] == "RUN" {
			runName := lineFields[2]
			if runName == testName {
				foundRun = i
				break
			}
			// Found a RUN line for another test (nested or interleaved)
			runLinesInBetween = true
		}
	}

	if foundRun != -1 {
		if !runLinesInBetween {
			// Clean block: No other RUN lines in between. Safe to truncate.
			// Remove from foundRun to end.
			cf.buffer = cf.buffer[:foundRun]
		} else {
			// Interleaved or nested.
			// Remove the PASS line (last element)
			if len(cf.buffer) > 0 {
				cf.buffer = cf.buffer[:len(cf.buffer)-1]
			}
			// Remove the RUN line (at foundRun)
			cf.buffer = append(cf.buffer[:foundRun], cf.buffer[foundRun+1:]...)
		}
	} else {
		// If we couldn't find the RUN line, but found a PASS line, remove the PASS line.
		if len(cf.buffer) > 0 {
			cf.buffer = cf.buffer[:len(cf.buffer)-1]
		}
	}
}

func (cf *ConsoleFilter) Flush() {
	// Process any remaining partial line
	if cf.incompleteLine != "" {
		cf.addLine(cf.incompleteLine)
		cf.incompleteLine = ""
	}

	// Show data race warning once at the start
	if cf.hasDataRace && !cf.shownRaceMsg {
		cf.output("⚠️  WARNING: DATA RACE detected")
		cf.shownRaceMsg = true
	}

	for _, line := range cf.buffer {
		cf.output(line)
	}
	cf.buffer = nil
}
