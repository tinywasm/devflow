package devflow_test

import "github.com/tinywasm/devflow"

import (
	"fmt"
	"strings"
	"testing"
)

func TestGo_SetLog(t *testing.T) {
	git, _ := devflow.NewGit()
	g, _ := devflow.NewGo(git)

	// Test that SetLog works
	called := false
	g.SetLog(func(args ...any) {
		called = true
	})

	// Call log to verify it works
	g.GetLog()("test")

	if !called {
		t.Error("Expected log function to be called")
	}
}

func TestGo_NewGo(t *testing.T) {
	git, _ := devflow.NewGit()
	g, _ := devflow.NewGo(git)

	if g == nil {
		t.Fatal("Expected NewGo to return non-nil")
	}

	if g.GetGit() != git {
		t.Error("Expected git handler to be set")
	}
}

// TestConsoleFilter validates that ConsoleFilter works correctly
func TestConsoleFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		excluded []string
	}{
		{
			name: "Shows test errors",
			input: `=== RUN   TestFail
    test.go:10: error message
--- FAIL: TestFail (0.00s)
FAIL`,
			expected: []string{"test.go:10: error message", "--- FAIL: TestFail"},
			excluded: []string{},
		},
		{
			name: "Filters passing tests",
			input: `=== RUN   TestPass
--- PASS: TestPass (0.00s)
=== RUN   TestFail
--- FAIL: TestFail (0.00s)
FAIL`,
			expected: []string{"--- FAIL: TestFail"},
			excluded: []string{"TestPass"},
		},
		{
			name: "Shows data race warning once",
			input: `WARNING: DATA RACE
WARNING: DATA RACE
--- FAIL: TestRace (0.00s)
    testing.go:1617: race detected during execution of test
FAIL`,
			expected: []string{"⚠️  WARNING: DATA RACE detected", "testing.go:1617"},
			excluded: []string{},
		},
		{
			name: "Skips stack traces",
			input: `--- FAIL: TestFail (0.00s)
      /usr/local/go/src/testing/testing.go:1934 +0x21c
    test.go:10: error
FAIL`,
			expected: []string{"test.go:10: error"},
			excluded: []string{"/usr/local/go"},
		},
		{
			name: "Real WASM test failure scenario - minimal output",
			input: `# github.com/tinywasm/jsvalue
package github.com/tinywasm/jsvalue: build constraints exclude all Go files in /home/cesar/Dev/Pkg/tinywasm/jsvalue
FAIL    github.com/tinywasm/jsvalue [setup failed]
FAIL
=== RUN   TestToJS
=== RUN   TestToJS/int32
    jsvalue_test.go:83: ToJS validation failed for int32
=== RUN   TestToJS/uint16
    jsvalue_test.go:83: ToJS validation failed for uint16
--- FAIL: TestToJS (0.01s)
    --- FAIL: TestToJS/int32 (0.00s)
    --- FAIL: TestToJS/uint16 (0.00s)
coverage: 92.5% of statements
exit with status 1
FAIL    github.com/tinywasm/jsvalue     0.462s
FAIL`,
			expected: []string{
				"--- FAIL: TestToJS (0.01s)",
				"--- FAIL: TestToJS/int32 (0.00s)",
				"jsvalue_test.go:83: ToJS validation failed for int32",
				"--- FAIL: TestToJS/uint16 (0.00s)",
				"jsvalue_test.go:83: ToJS validation failed for uint16",
			},
			excluded: []string{
				// Note: "# github.com/..." is kept because it shows which package has errors
				"package github.com/tinywasm/jsvalue: build constraints",
				"[setup failed]",
				"coverage:",
				"exit with status",
				"FAIL\tgithub.com/tinywasm/jsvalue",
			},
		},
		{
			name:     "Filters duplicate coverage from stdlib tests",
			input:    "ok  \tgithub.com/tinywasm/time\t0.504s\tcoverage: 96.8% of statements\n✅ vet ok, ✅ tests stdlib ok, ✅ race detection ok, ✅ coverage: 100%, ✅ tests wasm ok",
			expected: []string{"✅ vet ok", "✅ tests stdlib ok", "✅ race detection ok", "✅ coverage: 100%", "✅ tests wasm ok"},
			excluded: []string{"ok  \tgithub.com/tinywasm", "coverage: 96.8%", "0.504s"},
		},
		{
			name: "Real data race detection with stack traces",
			input: `=== RUN   TestRaceCondition
WARNING: DATA RACE
Read at 0x00c000018090 by goroutine 8:
  github.com/tinywasm/test.(*Counter).Get()
      /home/cesar/Dev/Pkg/tinywasm/test/counter.go:15 +0x38
  github.com/tinywasm/test.TestRaceCondition.func1()
      /home/cesar/Dev/Pkg/tinywasm/test/race_test.go:20 +0x2c

Previous write at 0x00c000018090 by goroutine 7:
  github.com/tinywasm/test.(*Counter).Inc()
      /home/cesar/Dev/Pkg/tinywasm/test/counter.go:10 +0x64
  github.com/tinywasm/test.TestRaceCondition()
      /home/cesar/Dev/Pkg/tinywasm/test/race_test.go:15 +0x7c

WARNING: DATA RACE
Read at 0x00c000018090 by goroutine 9:
  github.com/tinywasm/test.(*Counter).Get()
      /home/cesar/Dev/Pkg/tinywasm/test/counter.go:15 +0x38
WARNING: DATA RACE
--- FAIL: TestRaceCondition (0.00s)
    testing.go:1617: race detected during execution of test
FAIL`,
			expected: []string{
				"⚠️  WARNING: DATA RACE detected",
				"counter.go:15",
				"--- FAIL: TestRaceCondition",
				"testing.go:1617: race detected during execution of test",
			},
			excluded: []string{
				"Read at 0x00c",
				"Previous write",
				"goroutine 8:",
				"goroutine 7:",
				"+0x38",
				"+0x64",
			},
		},
		{
			name: "Nil pointer panic with stack trace",
			input: `=== RUN   TestNilPointer
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x5a2e45]

goroutine 6 [running]:
github.com/tinywasm/test.(*Handler).Process(...)
	/home/cesar/Dev/Pkg/tinywasm/test/handler.go:25
github.com/tinywasm/test.TestNilPointer(0xc0000a6b60)
	/home/cesar/Dev/Pkg/tinywasm/test/handler_test.go:12 +0x25
testing.tRunner(0xc0000a6b60, 0x5f4c28)
	/usr/local/go/src/testing/testing.go:1689 +0xfb
created by testing.(*T).Run in goroutine 1
	/usr/local/go/src/testing/testing.go:1742 +0x390
--- FAIL: TestNilPointer (0.00s)
FAIL`,
			expected: []string{
				"panic: runtime error: invalid memory address or nil pointer dereference",
				"handler.go:25",
				"--- FAIL: TestNilPointer",
				// Now we EXPECT these to be present because we are in panic mode
				"goroutine 6",
				"signal SIGSEGV",
			},
			excluded: []string{
				// converted to expected or just allowed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output []string
			filter := devflow.NewConsoleFilter(func(s string) {
				output = append(output, s)
			})

			filter.Add(tt.input)
			filter.Flush()

			result := strings.Join(output, "\n")

			for _, exp := range tt.expected {
				if !strings.Contains(result, exp) {
					t.Errorf("Expected %q, got:\n%s", exp, result)
				}
			}

			for _, exc := range tt.excluded {
				if strings.Contains(result, exc) {
					t.Errorf("Should NOT contain %q, got:\n%s", exc, result)
				}
			}
		})
	}
}

func TestShouldEnableWasm(t *testing.T) {
	tests := []struct {
		name      string
		nativeOut string
		wasmOut   string
		expected  bool
	}{
		{
			name:      "Client - Identical (Purely Native)",
			nativeOut: "github.com/tinywasm/client [a_test.go]\nbenchmark/err [syscall/js error]",
			wasmOut:   "github.com/tinywasm/client [a_test.go]",
			expected:  false,
		},
		{
			name:      "JSValue - WASM only (Native yields nothing functional)",
			nativeOut: "github.com/tinywasm/jsvalue [] []",
			wasmOut:   "github.com/tinywasm/jsvalue [jsvalue_test.go] []",
			expected:  true,
		},
		{
			name:      "Fetch - Dual (Additional WASM test file)",
			nativeOut: "github.com/tinywasm/fetch [] [stdlib_test.go]",
			wasmOut:   "github.com/tinywasm/fetch [] [stdlib_test.go wasm_test.go]",
			expected:  true,
		},
		{
			name:      "Real Client Scenario (with construction noise)",
			nativeOut: "package github.com/tinywasm/client/benchmark/shared\nimports syscall/js: build constraints exclude all Go files\ngithub.com/tinywasm/client [wasm_exec_test.go tinystring_test.go] []",
			wasmOut:   "github.com/tinywasm/client [wasm_exec_test.go tinystring_test.go] []",
			expected:  false,
		},
		{
			name:      "KVDB - Purely Native (no tags)",
			nativeOut: "github.com/tinywasm/kvdb [methods_test.go] []",
			wasmOut:   "github.com/tinywasm/kvdb [methods_test.go] []",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := devflow.ShouldEnableWasm(tt.nativeOut, tt.wasmOut)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
func TestEvaluateTestResults(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		output       string
		skipRace     bool
		expected     string
		expectedRan  bool
		expectedMsgs []string // Messages that MUST be present
		excludedMsgs []string // Messages that MUST NOT be present (anti-noise regression)
	}{
		{
			name:        "Pure Success",
			err:         nil,
			output:      "ok  github.com/mod 1.0s",
			expected:    "Passing",
			expectedRan: true,
			expectedMsgs: []string{
				"✅ tests stdlib ok",
				"✅ race detection ok",
			},
		},
		{
			name:        "Pure Success (Skip Race)",
			err:         nil,
			output:      "ok  github.com/mod 1.0s",
			skipRace:    true,
			expected:    "Passing",
			expectedRan: true,
			expectedMsgs: []string{
				"✅ tests stdlib ok",
				"✅ race detection skipped",
			},
			excludedMsgs: []string{
				"race detection ok",
			},
		},
		{
			name:        "Real Test Failure",
			err:         fmt.Errorf("exit 1"),
			output:      "--- FAIL: TestSomething\nFAIL  github.com/mod",
			expected:    "Failed",
			expectedRan: true,
			expectedMsgs: []string{
				"❌ Test errors found in testmod",
			},
		},
		{
			name:        "Build Failure",
			err:         fmt.Errorf("exit 2"),
			output:      "# github.com/mod\n[build failed]",
			expected:    "Failed",
			expectedRan: false,
			expectedMsgs: []string{
				"❌ Test errors found in testmod",
			},
		},
		{
			name: "Client Scenario: Partial Success (Native ok, subpackages tag-excluded)",
			err:  fmt.Errorf("exit 1"),
			output: "# github.com/tinywasm/client/benchmark/shared\n" +
				"package github.com/tinywasm/client/benchmark/shared\n" +
				"        imports syscall/js: build constraints exclude all Go files in /usr/local/go/src/syscall/js\n" +
				"FAIL\tgithub.com/tinywasm/client/benchmark/shared [setup failed]\n" +
				"# github.com/tinywasm/client/test\n" +
				"package github.com/tinywasm/client/benchmark/shared\n" +
				"        imports syscall/js: build constraints exclude all Go files in /usr/local/go/src/syscall/js\n" +
				"FAIL\tgithub.com/tinywasm/client/test [setup failed]\n" +
				"ok  \tgithub.com/tinywasm/client      4.417s\n" +
				"FAIL",
			expected:    "Passing",
			expectedRan: true,
			expectedMsgs: []string{
				"✅ tests stdlib ok", // Clean message only
				"✅ race detection ok",
			},
			excludedMsgs: []string{
				"(some subpackages excluded)", // Anti-regression check
			},
		},
		{
			name:        "WASM-only: Total Exclusion",
			err:         fmt.Errorf("exit 1"),
			output:      "matched no packages\nbuild constraints exclude all Go files",
			expected:    "Passing",
			expectedRan: false,
			// No messages expected for stdlib in this case, logic handles it upstream or returns empty msgs for this part
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, ran, msgs := devflow.EvaluateTestResults(tt.err, tt.output, "testmod", nil, tt.skipRace)
			if status != tt.expected {
				t.Errorf("Expected status %s, got %s", tt.expected, status)
			}
			if ran != tt.expectedRan {
				t.Errorf("Expected ran %v, got %v", tt.expectedRan, ran)
			}

			// Verify messages
			for _, exp := range tt.expectedMsgs {
				found := false
				for _, msg := range msgs {
					if msg == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected message %q not found in %v", exp, msgs)
				}
			}

			// Verify exclusions (Anti-noise)
			for _, exc := range tt.excludedMsgs {
				for _, msg := range msgs {
					if strings.Contains(msg, exc) {
						t.Errorf("Found excluded noise message %q in %v", exc, msgs)
					}
				}
			}
		})
	}
}

func TestFindSlowestTest(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		threshold float64
		expName   string
		expDur    float64
	}{
		{
			name:      "Empty output",
			output:    "",
			threshold: 1.0,
			expName:   "",
			expDur:    0,
		},
		{
			name: "All tests below threshold",
			output: `=== RUN   TestSlow
--- PASS: TestSlow (0.50s)
=== RUN   TestFast
--- PASS: TestFast (0.10s)`,
			threshold: 1.0,
			expName:   "",
			expDur:    0,
		},
		{
			name: "One test above threshold",
			output: `=== RUN   TestSlow
--- PASS: TestSlow (2.50s)
=== RUN   TestFast
--- PASS: TestFast (0.10s)`,
			threshold: 1.0,
			expName:   "TestSlow",
			expDur:    2.5,
		},
		{
			name: "Multiple packages, multiple slow tests",
			output: `--- PASS: TestPkgA_Slow (3.00s)
--- PASS: TestPkgB_Slower (4.50s)
--- PASS: TestPkgC_Fast (0.10s)`,
			threshold: 2.0,
			expName:   "TestPkgB_Slower",
			expDur:    4.5,
		},
		{
			name: "Failure still counts as slow",
			output: `=== RUN   TestFail
--- FAIL: TestFail (5.00s)
FAIL`,
			threshold: 2.0,
			expName:   "TestFail",
			expDur:    5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, dur := devflow.FindSlowestTest(tt.output, tt.threshold)
			if name != tt.expName {
				t.Errorf("Expected name %q, got %q", tt.expName, name)
			}
			if dur != tt.expDur {
				t.Errorf("Expected duration %f, got %f", tt.expDur, dur)
			}
		})
	}
}

func TestHasTimeoutFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{"No args", nil, false},
		{"Unrelated flags", []string{"-v", "-run", "TestFoo"}, false},
		{"-timeout=30s", []string{"-v", "-timeout=30s"}, true},
		{"-timeout 30s", []string{"-timeout", "30s"}, true},
		{"-test.timeout=30s", []string{"-test.timeout=30s"}, true},
		{"-test.timeout 30s", []string{"-test.timeout", "30s"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := devflow.HasTimeoutFlag(tt.args); got != tt.expected {
				t.Errorf("devflow.HasTimeoutFlag(%v) = %v, want %v", tt.args, got, tt.expected)
			}
		})
	}
}

func TestFindTimedOutTests(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "No timeout",
			output:   "--- PASS: TestFoo (0.01s)\nok  github.com/mod",
			expected: nil,
		},
		{
			name: "Go timeout with running tests section",
			output: `=== RUN   TestSlowOperation
panic: test timed out after 30s
        running tests:
                TestSlowOperation (30s)

goroutine 1 [running]:`,
			expected: []string{"TestSlowOperation"},
		},
		{
			name: "Fallback: last RUN without PASS/FAIL",
			output: `=== RUN   TestFastOne
--- PASS: TestFastOne (0.01s)
=== RUN   TestHanging
panic: test timed out after 30s

goroutine 1 [running]:`,
			expected: []string{"TestHanging"},
		},
		{
			name: "Multiple running tests",
			output: `panic: test timed out after 30s
        running tests:
                TestA (30s)
                TestB (25s)

goroutine 1 [running]:`,
			expected: []string{"TestA", "TestB"},
		},
		{
			name: "Process killed (WASM): no panic message, only RUN lines",
			output: `=== RUN   TestRenderToBody
=== RUN   TestRenderToBody/Render_ViewRenderer_to_body`,
			expected: []string{"TestRenderToBody/Render_ViewRenderer_to_body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := devflow.FindTimedOutTests(tt.output)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("Expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.expected) {
				t.Fatalf("Expected %d tests, got %d: %v", len(tt.expected), len(got), got)
			}
			for i, name := range tt.expected {
				if got[i] != name {
					t.Errorf("Expected [%d]=%q, got %q", i, name, got[i])
				}
			}
		})
	}
}
