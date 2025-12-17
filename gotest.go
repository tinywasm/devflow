package devflow

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Test executes the test suite for the project
func (g *Go) Test(verbose bool) (string, error) {
	// Detect Module Name
	moduleName, err := getModuleName(".")
	if err != nil {
		return "", fmt.Errorf("error: %v", err)
	}

	if verbose {
		g.log("Running tests, race detection, vet and coverage analysis for", moduleName)
	}

	// Initialize Status
	testStatus := "Failed"
	coveragePercent := "0"
	raceStatus := "Detected"
	vetStatus := "Issues"

	var msgs []string
	addMsg := func(ok bool, msg string) {
		symbol := "✅"
		if !ok {
			symbol = "❌"
		}
		msgs = append(msgs, fmt.Sprintf("%s %s", symbol, msg))
	}

	quiet := !verbose

	// Parallel Phase 1: Vet + Test file detection
	var wg1 sync.WaitGroup
	var vetOutput string
	var vetErr error
	var hasTestFiles bool
	var enableWasmTests bool

	wg1.Add(3)

	// Go Vet (async)
	go func() {
		defer wg1.Done()
		vetOutput, vetErr = RunCommand("go", "vet", ".")
	}()

	// Check for test files (async)
	go func() {
		defer wg1.Done()
		out, _ := RunCommand("find", ".", "-type", "f", "-name", "*_test.go")
		hasTestFiles = len(out) > 0
	}()

	// Check for WASM test files by build tags ONLY (async)
	go func() {
		defer wg1.Done()
		// Only check for wasm build tag in test files - don't rely on file names
		// as files like "wasm_exec_test.go" are normal tests about WASM, not WASM tests
		buildTagOut, _ := RunShellCommand("grep -l '^//go:build.*wasm' *_test.go 2>/dev/null || true")
		if len(buildTagOut) > 0 {
			enableWasmTests = true
			if !quiet {
				g.log("Detected WASM build tags...")
			}
		}
	}()

	wg1.Wait()

	// Process vet results
	if vetErr != nil {
		// Check if it's just "no packages" error (WASM-only projects)
		if strings.Contains(vetOutput, "matched no packages") ||
			strings.Contains(vetOutput, "no packages to vet") ||
			strings.Contains(vetOutput, "build constraints exclude all Go files") {
			vetStatus = "OK"
			addMsg(true, "vet ok")
		} else {
			vetStatus = "Issues"
			// Filter unsafe.Pointer warnings
			lines := strings.Split(vetOutput, "\n")
			var filteredLines []string
			for _, line := range lines {
				if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") { // Ignore comments/empty
					continue
				}
				if !strings.Contains(line, "possible misuse of unsafe.Pointer") {
					filteredLines = append(filteredLines, line)
				}
			}

			if len(filteredLines) > 0 {
				if !quiet {
					g.log("go vet failed:")
					for _, l := range filteredLines {
						g.log(l)
					}
				}
				addMsg(false, "vet issues found")
			} else {
				vetStatus = "OK"
				addMsg(true, "vet ok")
			}
		}
	} else {
		vetStatus = "OK"
		addMsg(true, "vet ok")
	}

	if hasTestFiles {
		if !quiet {
			g.log("Running Go tests with race detection and coverage...")
		}

		// Run tests with race detection AND coverage in a single command
		// Running them in parallel causes cache conflicts
		var testErr error
		var testOutput string

		testCmd := exec.Command("go", "test", "-race", "-cover", ".")

		var testFilterCallback func(string)
		if !quiet {
			testFilterCallback = func(s string) {
				fmt.Println(s)
			}
		}
		testFilter := NewConsoleFilter(quiet, testFilterCallback)

		testBuffer := &bytes.Buffer{}
		testBufferUnfiltered := &bytes.Buffer{} // Capture unfiltered output for error reporting

		testPipe := &paramWriter{
			write: func(p []byte) (n int, err error) {
				s := string(p)
				testBuffer.Write(p)
				testBufferUnfiltered.Write(p) // Always capture complete output
				testFilter.Add(s)
				return len(p), nil
			},
		}

		testCmd.Stdout = testPipe
		testCmd.Stderr = testPipe
		testErr = testCmd.Run()
		testFilter.Flush()

		testOutput = testBufferUnfiltered.String() // Use unfiltered output for error detection

		// Process test results
		stdTestsRan := false
		if testErr != nil {
			// Check if it's a WASM-only package (build constraints exclude all files)
			if strings.Contains(testOutput, "matched no packages") ||
				strings.Contains(testOutput, "build constraints exclude all Go files") {
				testStatus = "Passing"
				raceStatus = "Clean"
				// Ensure WASM tests are enabled for WASM-only packages
				enableWasmTests = true
				if !quiet {
					g.log("WASM-only package detected, skipping stdlib tests...")
				}
			} else {
				// Real test failure - show only error lines in quiet mode
				if quiet {
					// Extract and show FAIL lines and error messages
					lines := strings.Split(testOutput, "\n")
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						// Show FAIL lines, error messages, and test file references
						if strings.HasPrefix(trimmed, "FAIL") ||
							strings.HasPrefix(trimmed, "--- FAIL:") ||
							strings.Contains(line, "_test.go:") ||
							strings.Contains(trimmed, "Error:") ||
							strings.Contains(trimmed, "panic:") {
							fmt.Println(line)
						}
					}
				}
				addMsg(false, fmt.Sprintf("Test errors found in %s", moduleName))
				testStatus = "Failed"
				raceStatus = "Detected"
				stdTestsRan = true
			}
		} else {
			testStatus = "Passing"
			raceStatus = "Clean"
			addMsg(true, "tests stdlib ok")
			addMsg(true, "race detection ok")
			stdTestsRan = true
		}

		// Process coverage results (from the same test run)
		if stdTestsRan {
			coveragePercent = calculateAverageCoverage(testOutput)
			if coveragePercent != "0" {
				addMsg(true, "coverage: "+coveragePercent+"%")
			}
		}

		// WASM Tests
		if enableWasmTests {
			if !quiet {
				g.log("Running WASM tests...")
			}

			if err := g.installWasmBrowserTest(quiet); err != nil {
				if !quiet {
					g.log("⚠️  wasmbrowsertest setup failed:", err)
				}
				addMsg(false, "WASM tests skipped (setup failed)")
			} else {
				execArg := "wasmbrowsertest -quiet"
				testArgs := []string{"test", "-exec", execArg, "-cover", "."}
				if !quiet {
					execArg = "wasmbrowsertest"
					testArgs = []string{"test", "-exec", execArg, "-v", "-cover", "."}
				}

				wasmCmd := exec.Command("go", testArgs...)
				wasmCmd.Env = os.Environ()
				wasmCmd.Env = append(wasmCmd.Env, "GOOS=js", "GOARCH=wasm")

				var wasmOut bytes.Buffer
				var wasmOutUnfiltered bytes.Buffer // Capture unfiltered output for error reporting

				var wasmFilterCallback func(string)
				if !quiet {
					wasmFilterCallback = func(s string) {
						fmt.Println(s)
					}
				}
				wasmFilter := NewConsoleFilter(quiet, wasmFilterCallback)
				wasmPipe := &paramWriter{
					write: func(p []byte) (n int, err error) {
						s := string(p)
						wasmOut.Write(p)
						wasmOutUnfiltered.Write(p) // Always capture complete output
						wasmFilter.Add(s)
						return len(p), nil
					},
				}

				wasmCmd.Stdout = wasmPipe
				wasmCmd.Stderr = wasmPipe

				err := wasmCmd.Run()
				wasmFilter.Flush()

				wOutput := wasmOutUnfiltered.String() // Use unfiltered output

				if err != nil {
					// WASM test failure - show only error lines in quiet mode
					if quiet {
						lines := strings.Split(wOutput, "\n")
						for _, line := range lines {
							trimmed := strings.TrimSpace(line)
							// Show FAIL lines, error messages, and test file references
							if strings.HasPrefix(trimmed, "FAIL") ||
								strings.HasPrefix(trimmed, "--- FAIL:") ||
								strings.Contains(line, "_test.go:") ||
								strings.Contains(trimmed, "Error:") ||
								strings.Contains(trimmed, "panic:") {
								fmt.Println(line)
							}
						}
					}
					addMsg(false, "tests wasm failed")
					testStatus = "Failed"
				} else {
					addMsg(true, "tests wasm ok")
					if testStatus != "Failed" {
						testStatus = "Passing"
					}
					wCov := calculateAverageCoverage(wOutput)
					if wCov != "0" {
						coveragePercent = wCov
						if !stdTestsRan {
							addMsg(true, "coverage: "+coveragePercent+"%")
						}
					}
				}
			}
		}

	} else {
		addMsg(true, fmt.Sprintf("no test files found in %s", moduleName))
		testStatus = "Passing"
		coveragePercent = "0"
	}

	// Badges
	if !quiet {
		g.log("Updating badges...")
	}
	licenseType := "MIT"
	if checkFileExists("LICENSE") {
		// naive check
	}
	goVer := getGoVersion()

	if err := updateBadges("README.md", licenseType, goVer, testStatus, coveragePercent, raceStatus, vetStatus, quiet); err != nil {
		if !quiet {
			g.log("Error updating badges:", err)
		}
	}

	// Final Summary
	allPassed := testStatus == "Passing" && raceStatus == "Clean" && vetStatus == "OK"

	if quiet && allPassed {
		return strings.Join(msgs, ", "), nil
	} else {
		if quiet {
			return strings.Join(msgs, ", "), nil
		} else {
			return strings.Join(msgs, "\n"), nil
		}
	}
}

type paramWriter struct {
	write func(p []byte) (n int, err error)
}

func (p *paramWriter) Write(b []byte) (n int, err error) {
	return p.write(b)
}

func calculateAverageCoverage(output string) string {
	lines := strings.Split(output, "\n")
	var total float64
	var count int

	re := regexp.MustCompile(`coverage:\s+(\d+(\.\d+)?)%`)

	for _, line := range lines {
		if strings.Contains(line, "[no test files]") {
			continue
		}
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			val, err := strconv.ParseFloat(matches[1], 64)
			if err == nil && val > 0 {
				total += val
				count++
			}
		}
	}

	if count == 0 {
		return "0"
	}
	return fmt.Sprintf("%.0f", total/float64(count))
}

func (g *Go) installWasmBrowserTest(quiet bool) error {
	if _, err := RunCommandSilent("which", "wasmbrowsertest"); err == nil {
		return nil
	}
	if !quiet {
		g.log("Installing wasmbrowsertest from tinywasm fork...")
	}
	_, err := RunCommand("go", "install", "github.com/tinywasm/wasmbrowsertest@latest")
	if err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}
	return nil
}
