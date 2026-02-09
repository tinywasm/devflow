package devflow

import (
	"strings"
	"testing"
)

func TestConsoleFilter_Quiet(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(record)

	// Case 1: Passing test
	cf.Add("=== RUN   TestPass\n")
	cf.Add("some log\n")
	cf.Add("--- PASS: TestPass (0.01s)\n")

	if len(output) != 0 {
		t.Errorf("Expected no output for passing test, got: %v", output)
	}

	// Case 2: Failing test
	cf.Add("=== RUN   TestFail\n")
	cf.Add("    jsvalue_test.go:83: ToJS validation failed for int16\n")
	cf.Add("--- FAIL: TestFail (0.02s)\n")

	// Manually flush since we delay flushing in actual implementation
	cf.Flush()

	// Should have flushed: Run, error log, Fail
	expected := []string{
		"=== RUN   TestFail",
		"    jsvalue_test.go:83: ToJS validation failed for int16",
		"--- FAIL: TestFail (0.02s)",
	}
	if len(output) != len(expected) {
		t.Fatalf("Expected %d output lines, got %d. Output: %v", len(expected), len(output), output)
	}
	for i := range expected {
		if output[i] != expected[i] {
			t.Errorf("Line %d mismatch: expected %q, got %q", i, expected[i], output[i])
		}
	}

	// Reset output
	output = nil

	// Case 3: Nested passing subtests in a passing parent
	cf.Add("=== RUN   TestParent\n")
	cf.Add("=== RUN   TestParent/ChildPass\n")
	cf.Add("    --- PASS: TestParent/ChildPass (0.00s)\n")
	cf.Add("--- PASS: TestParent (0.01s)\n")

	if len(output) != 0 {
		t.Errorf("Expected no output for passing parent with subtests, got: %v", output)
	}

	// Case 4: Nested passing subtests in a failing parent
	output = nil
	cf.Add("=== RUN   TestFailParent\n")
	cf.Add("=== RUN   TestFailParent/ChildPass\n")
	cf.Add("    --- PASS: TestFailParent/ChildPass (0.00s)\n") // filtered out
	cf.Add("=== RUN   TestFailParent/ChildFail\n")
	cf.Add("    jsvalue_test.go:83: ToJS validation failed for uint\n")
	cf.Add("    --- FAIL: TestFailParent/ChildFail (0.00s)\n")
	cf.Add("--- FAIL: TestFailParent (0.05s)\n")

	// Manually flush
	cf.Flush()

	// Check output
	// We expect: Run Parent, Run ChildFail, error log, Fail ChildFail, Fail Parent
	// We expect NO: Run ChildPass, Pass ChildPass

	unexpected := "ChildPass"
	for _, line := range output {
		if strings.Contains(line, unexpected) {
			t.Errorf("Output should not contain '%s', got: %v", unexpected, output)
		}
	}

	if len(output) < 4 {
		t.Errorf("Expected output to contain failure logs, got: %v", output)
	}

	// Verify error message is present
	foundError := false
	for _, line := range output {
		if strings.Contains(line, "ToJS validation failed for uint") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Expected to find error message in output, got: %v", output)
	}
}

func TestConsoleFilter_FilterNoise(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(record)

	// Test filtering of various noise messages
	cf.Add("go: warning: \"./...\" matched no packages\n")
	cf.Add("no packages to test\n")
	cf.Add("✅ All tests passed!\n")
	cf.Add("Badges saved to docs/img/badges.svg\n")

	if len(output) != 0 {
		t.Errorf("Expected all noise to be filtered, got: %v", output)
	}

	// Test that actual test output is not filtered
	cf.Add("=== RUN   TestSomething\n")
	cf.Add("--- FAIL: TestSomething (0.01s)\n")
	cf.Flush()

	if len(output) != 2 {
		t.Errorf("Expected test output to not be filtered, got: %v", output)
	}
}

// TestConsoleFilter_ErrorMessageWithoutKeywords reproduces the bug where
// error messages without keywords (fail/error/panic/race) are incorrectly filtered.
// Example: "time_test.go:45: got unexpected value" is filtered because it doesn't
// contain "fail" or "error" etc.
func TestConsoleFilter_ErrorMessageWithoutKeywords(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(record)

	// Error message WITHOUT keywords like "fail", "error", "panic", "race"
	cf.Add("=== RUN   TestFormatTimeWithNumericString\n")
	cf.Add("    time_test.go:45: got unexpected value 123\n")
	cf.Add("--- FAIL: TestFormatTimeWithNumericString (0.00s)\n")
	cf.Add("FAIL\n")
	cf.Add("FAIL\tgithub.com/tinywasm/time\t0.015s\n")
	cf.Flush()

	// The error message SHOULD be shown even without keywords
	foundError := false
	for _, line := range output {
		if strings.Contains(line, "got unexpected value") {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Errorf("BUG REPRODUCED: Error message without keywords is filtered out, got: %v", output)
	}
}

func TestConsoleFilter_AlwaysShowDebug(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(record)

	// DEBUG message should NOT be filtered even if the test passes
	cf.Add("=== RUN   TestDebug\n")
	cf.Add("DEBUG: connecting to database\n")
	cf.Add("--- PASS: TestDebug (0.01s)\n")
	cf.Flush()

	foundDebug := false
	for _, line := range output {
		if strings.Contains(line, "DEBUG:") {
			foundDebug = true
			break
		}
	}

	if !foundDebug {
		t.Errorf("Expected DEBUG message to be shown, but it was filtered out. Output: %v", output)
	}
}

func TestConsoleFilter_BufferFragmentation(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(record)

	// Simulate fragmented writes WITH newlines
	// Write "=== RUN TestFrag\n"
	// Write "--- PASS: TestFrag (0"
	// Write ".00s)\n"

	cf.Add("=== RUN   TestFrag\n")  // Complete line
	cf.Add("--- PASS: TestFrag (0") // Incomplete
	cf.Add(".00s)\n")               // Complete the previous line
	cf.Flush()

	// If properly handled (buffer partial lines), the PASS line should be reconstructed
	// and trigger the removal of the RUN line.
	// If NOT properly handled, we will see "=== RUN   TestFrag" and maybe garbage from the fragmented PASS line.

	if len(output) != 0 {
		t.Errorf("Expected fragmented PASS line to still trigger filtering. Got: %v", output)
	}
}

func TestConsoleFilter_OrphanedPassLine(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(record)

	// Simulate an orphaned PASS line (e.g. RUN line was flushed earlier or missing)
	// The filter should still remove this line even if it can't find the RUN line.
	cf.Add("--- PASS: TestOrphan (0.00s)\n")
	cf.Flush()

	if len(output) != 0 {
		t.Errorf("Expected orphaned PASS line to be filtered. Got: %v", output)
	}
}

func TestConsoleFilter_PanicMode(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(record)

	// 1. Normal filtering phase: "goroutine" lines are normally noise
	cf.Add("goroutine 1 [running]:\n")

	// Verify it was filtered (buffer might hold it if it thinks it's part of a block,
	// but goroutine lines are explicitly skipped in addLine)
	// Actually, let's flush to be sure, though typically flush dumps buffer.
	// But `goroutine` lines return early in `addLine`, so they don't even hit the buffer.
	// So we can check output now? No, `NewConsoleFilter` output callback is immediate for some things logic-wise?
	// `addLine` calls `output` directly for DEBUG, but for others it appends to buffer.
	// Wait, `goroutine` lines invoke `return` in `addLine`, so they are just dropped.
	// They are NOT added to buffer.

	if len(output) != 0 {
		t.Errorf("Expected initial noise to be filtered, got: %v", output)
	}

	// 2. Trigger Panic Mode
	panicMsg := "panic: test timed out after 30s\n"
	cf.Add(panicMsg)

	// 3. Post-panic noise (should be preserved)
	// These lines would normally be filtered
	traceLine1 := "goroutine 442 [running]:\n"
	traceLine2 := "\t/usr/local/go/src/testing/testing.go:2682 +0x345\n"

	cf.Add(traceLine1)
	cf.Add(traceLine2)

	cf.Flush()

	// Verification
	// We expect: panicMsg, traceLine1, traceLine2
	expectedCount := 3
	if len(output) != expectedCount {
		t.Fatalf("Expected %d lines in panic mode, got %d. Output: %v", expectedCount, len(output), output)
	}

	if output[0] != "panic: test timed out after 30s" {
		t.Errorf("Expected panic message, got: %q", output[0])
	}
	if output[1] != "goroutine 442 [running]:" {
		t.Errorf("Expected trace line 1, got: %q", output[1])
	}
	if output[2] != "\t/usr/local/go/src/testing/testing.go:2682 +0x345" {
		t.Errorf("Expected trace line 2, got: %q", output[2])
	}
}
