package devflow

import (
	"fmt"
	"strings"
)

// ConsoleFilter buffers console output and filters out passing tests when in quiet mode.
type ConsoleFilter struct {
	buffer []string
	quiet  bool
	output func(string) // callback to write output
}

func NewConsoleFilter(quiet bool, output func(string)) *ConsoleFilter {
	if output == nil {
		output = func(s string) { fmt.Printf("%s\n", s) }
	}
	return &ConsoleFilter{
		quiet:  quiet,
		output: output,
	}
}

func (cf *ConsoleFilter) Add(input string) {
	// Split input by newlines to ensure we handle line-by-line filtering
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		cf.addLine(line)
	}
}

func (cf *ConsoleFilter) addLine(line string) {
	if !cf.quiet {
		cf.output(line)
		return
	}

	// In quiet mode, filter out noise
	// Skip "go: warning" messages about matched no packages
	if strings.HasPrefix(line, "go: warning:") {
		return
	}
	// Skip "no packages to test" messages
	if strings.Contains(line, "no packages to test") {
		return
	}
	// Skip "[no test files]" messages
	if strings.Contains(line, "[no test files]") {
		return
	}
	// Skip "?" lines (package status without tests)
	if strings.HasPrefix(line, "?") && strings.Contains(line, "[no test files]") {
		return
	}
	// Skip wasmbrowsertest success messages (we have our own summary)
	if strings.Contains(line, "✅ All tests passed!") || strings.Contains(line, "All tests passed!") {
		return
	}
	// Skip wasmbrowsertest failure messages (we have our own summary)
	if strings.Contains(line, "❌ WASM tests failed") || strings.Contains(line, "WASM tests failed") {
		return
	}
	// Skip "Badges saved to" messages in quiet mode
	if strings.Contains(line, "Badges saved to") {
		return
	}
	// Skip t.Log messages from passing tests (e.g., "FormatDate tests passed")
	if strings.Contains(line, "tests passed") {
		return
	}
	// Skip t.Log messages that are just informational (contain .go: but not error/fail keywords)
	trimmed := strings.TrimSpace(line)
	if trimmed != line && strings.Contains(trimmed, ".go:") {
		// This is an indented line with file:line format (t.Log output)
		// Only keep it if it contains error/failure keywords
		lower := strings.ToLower(trimmed)
		if !strings.Contains(lower, "fail") &&
			!strings.Contains(lower, "error") &&
			!strings.Contains(lower, "panic") {
			// This is just informational log, skip it in quiet mode
			return
		}
	}

	// Always print global markers immediately and flush buffer
	// FAIL (global), PASS (global), ok, coverage, etc.
	// These usually appear at the very end of the test suite.
	if strings.HasPrefix(line, "FAIL") ||
		strings.HasPrefix(line, "PASS") ||
		strings.HasPrefix(line, "coverage:") ||
		strings.HasPrefix(line, "pkg:") ||
		strings.HasPrefix(line, "ok") ||
		strings.HasPrefix(line, "panic:") ||
		strings.HasPrefix(line, "exit status") {
		cf.Flush()

		// In quiet mode, suppress purely global summary lines "PASS" and "FAIL"
		// because gotest.go handles its own summary.
		return
	}

	// NOTE: We do NOT flush on individual "--- FAIL:" lines.
	// We buffer them. This allows us to keep the context (logs) of the failing test.
	// Since passing tests are removed from the buffer, the buffer will primarily contain
	// failing tests' logs (and currently running tests' logs).
	// We flush at the end (triggered by global markers).

	cf.buffer = append(cf.buffer, line)

	// If a test passed, remove its logs from buffer.
	// "--- PASS: TestName (0.00s)"
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
	if searchStart < 0 {
		return
	}

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
	for _, line := range cf.buffer {
		cf.output(line)
	}
	cf.buffer = nil
}
