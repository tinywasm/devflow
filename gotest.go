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
		vetOutput, vetErr = RunCommand("go", "vet", "./...")
	}()

	// Check for test files (async)
	go func() {
		defer wg1.Done()
		out, _ := RunCommand("find", ".", "-type", "f", "-name", "*_test.go")
		hasTestFiles = len(out) > 0
	}()

	// Check for WASM test files or build tags (async)
	go func() {
		defer wg1.Done()
		// Check file names
		wasmTestOut, _ := RunCommand("sh", "-c", "find . -type f \\( -name '*Wasm*_test.go' -o -name '*wasm*_test.go' \\) 2>/dev/null")
		if len(wasmTestOut) > 0 {
			enableWasmTests = true
			if !quiet {
				g.log("Detected WASM test files by name...")
			}
			return
		}
		// Check for wasm build tag in test files
		buildTagOut, _ := RunCommand("sh", "-c", "grep -l '^//go:build.*wasm' *_test.go 2>/dev/null || true")
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
		if strings.Contains(vetOutput, "matched no packages") || strings.Contains(vetOutput, "no packages to vet") {
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
				fmt.Println("go vet failed:")
				for _, l := range filteredLines {
					fmt.Println(l)
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

		// Parallel Phase 2: Tests with race + Coverage
		var wg2 sync.WaitGroup
		var testErr error
		var testOutput string
		var coverageOutput string
		var coverageErr error

		wg2.Add(2)

		// Tests with race detection (async)
		go func() {
			defer wg2.Done()
			testCmd := exec.Command("go", "test", "-race", "./...")

			testFilter := NewConsoleFilter(quiet, func(s string) {
				if quiet {
					fmt.Println(s)
				} else {
					fmt.Println(s)
				}
			})

			testBuffer := &bytes.Buffer{}

			testPipe := &paramWriter{
				write: func(p []byte) (n int, err error) {
					s := string(p)
					testBuffer.Write(p)
					testFilter.Add(s)
					return len(p), nil
				},
			}

			testCmd.Stdout = testPipe
			testCmd.Stderr = testPipe
			testErr = testCmd.Run()
			testFilter.Flush()

			testOutput = testBuffer.String()
		}()

		// Coverage (async)
		go func() {
			defer wg2.Done()
			if !quiet {
				g.log("Calculating coverage...")
			}
			coverageOutput, coverageErr = RunCommand("go", "test", "-cover", "./...")
		}()

		wg2.Wait()

		// Process test results
		stdTestsRan := false
		if testErr != nil {
			if strings.Contains(testOutput, "matched no packages") {
				testStatus = "Passing"
				raceStatus = "Clean"
				if !enableWasmTests {
					enableWasmTests = true
					if !quiet {
						g.log("No standard tests found, auto-enabling WASM tests...")
					}
				}
			} else {
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

		// Process coverage results
		if stdTestsRan {
			if coverageErr == nil {
				coveragePercent = calculateAverageCoverage(coverageOutput)
				if coveragePercent != "0" {
					addMsg(true, "coverage: "+coveragePercent+"%")
				}
			} else if strings.Contains(coverageOutput, "matched no packages") {
				// ignore
			} else {
				addMsg(false, "Failed to calculate coverage")
			}
		}

		// WASM Tests
		if enableWasmTests {
			if !quiet {
				g.log("Running WASM tests...")
			}

			if err := installWasmBrowserTest(); err != nil {
				fmt.Printf("⚠️  wasmbrowsertest setup failed: %v\n", err)
				addMsg(false, "WASM tests skipped (setup failed)")
			} else {
				execArg := "wasmbrowsertest -quiet"
				if !quiet {
					execArg = "wasmbrowsertest"
				}

				wasmCmd := exec.Command("go", "test", "-exec", execArg, "-v", "-cover", ".")
				wasmCmd.Env = os.Environ()
				wasmCmd.Env = append(wasmCmd.Env, "GOOS=js", "GOARCH=wasm")

				var wasmOut bytes.Buffer

				wasmFilter := NewConsoleFilter(quiet, nil)
				wasmPipe := &paramWriter{
					write: func(p []byte) (n int, err error) {
						s := string(p)
						wasmOut.Write(p)
						wasmFilter.Add(s)
						return len(p), nil
					},
				}

				wasmCmd.Stdout = wasmPipe
				wasmCmd.Stderr = wasmPipe

				err := wasmCmd.Run()
				wasmFilter.Flush()

				wOutput := wasmOut.String()

				if err != nil {
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

func installWasmBrowserTest() error {
	if _, err := RunCommandSilent("which", "wasmbrowsertest"); err == nil {
		return nil
	}
	fmt.Println("Installing wasmbrowsertest from tinywasm fork...")

	tmpDir, err := os.MkdirTemp("", "wasmbrowsertest-install")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("Cloning repository...")
	_, err = RunCommand("git", "clone", "--depth", "1", "https://github.com/tinywasm/wasmbrowsertest.git", tmpDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	fmt.Println("Building and installing...")
	_, err = RunCommand("go", "install", ".")
	if err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}
	return nil
}
