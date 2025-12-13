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

	cf := NewConsoleFilter(true, record)

	// Case 1: Passing test
	cf.Add("=== RUN   TestPass")
	cf.Add("some log")
	cf.Add("--- PASS: TestPass (0.01s)")

	if len(output) != 0 {
		t.Errorf("Expected no output for passing test, got: %v", output)
	}

	// Case 2: Failing test
	cf.Add("=== RUN   TestFail")
	cf.Add("    jsvalue_test.go:83: ToJS validation failed for int16")
	cf.Add("--- FAIL: TestFail (0.02s)")

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
	cf.Add("=== RUN   TestParent")
	cf.Add("=== RUN   TestParent/ChildPass")
	cf.Add("    --- PASS: TestParent/ChildPass (0.00s)")
	cf.Add("--- PASS: TestParent (0.01s)")

	if len(output) != 0 {
		t.Errorf("Expected no output for passing parent with subtests, got: %v", output)
	}

	// Case 4: Nested passing subtests in a failing parent
	output = nil
	cf.Add("=== RUN   TestFailParent")
	cf.Add("=== RUN   TestFailParent/ChildPass")
	cf.Add("    --- PASS: TestFailParent/ChildPass (0.00s)") // filtered out
	cf.Add("=== RUN   TestFailParent/ChildFail")
	cf.Add("    jsvalue_test.go:83: ToJS validation failed for uint")
	cf.Add("    --- FAIL: TestFailParent/ChildFail (0.00s)")
	cf.Add("--- FAIL: TestFailParent (0.05s)")

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

func TestConsoleFilter_Verbose(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(false, record)
	cf.Add("=== RUN   TestPass")
	cf.Add("log")
	cf.Add("--- PASS: TestPass")

	if len(output) != 3 {
		t.Errorf("Expected 3 lines in verbose mode, got %d", len(output))
	}
}

func TestConsoleFilter_FilterNoise(t *testing.T) {
	var output []string
	record := func(s string) {
		output = append(output, s)
	}

	cf := NewConsoleFilter(true, record)

	// Test filtering of various noise messages
	cf.Add("go: warning: \"./...\" matched no packages")
	cf.Add("no packages to test")
	cf.Add("✅ All tests passed!")
	cf.Add("Badges saved to docs/img/badges.svg")

	if len(output) != 0 {
		t.Errorf("Expected all noise to be filtered, got: %v", output)
	}

	// Test that actual test output is not filtered
	cf.Add("=== RUN   TestSomething")
	cf.Add("--- FAIL: TestSomething (0.01s)")
	cf.Flush()

	if len(output) != 2 {
		t.Errorf("Expected test output to not be filtered, got: %v", output)
	}
}
