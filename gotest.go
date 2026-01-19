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
func (g *Go) Test() (string, error) {
	// Detect Module Name
	moduleName, err := getModuleName(".")
	if err != nil {
		return "", fmt.Errorf("error: %v", err)
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

	// Parallel Phase 1: Vet + WASM detection
	var wg1 sync.WaitGroup
	var vetOutput string
	var vetErr error
	var enableWasmTests bool

	wg1.Add(2)

	// Go Vet (async)
	go func() {
		defer wg1.Done()
		vetOutput, vetErr = RunCommand("go", "vet", "./...")
	}()

	// Check for WASM test files by build tags ONLY (async)
	go func() {
		defer wg1.Done()
		// Check for wasm build tag in test files, excluding negated tags (!wasm)
		// First find files with wasm tag, then exclude those with !wasm
		buildTagOut, _ := RunShellCommand("grep -l '^//go:build.*wasm' *_test.go 2>/dev/null | while read f; do grep -q '!wasm' \"$f\" || echo \"$f\"; done")
		if strings.TrimSpace(buildTagOut) != "" {
			enableWasmTests = true
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

	// Run tests with race detection AND coverage in a single command
	// go test ./... automatically discovers all packages with tests
	var testErr error
	var testOutput string

	testCmd := exec.Command("go", "test", "-race", "-cover", "-count=1", "./...")

	testBuffer := &bytes.Buffer{}

	testFilter := NewConsoleFilter(nil)

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
			g.log("WASM-only package detected, skipping stdlib tests...")
		} else {
			// Real test failure - ConsoleFilter already filtered the output in quiet mode
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

		if err := g.installWasmBrowserTest(); err != nil {

			addMsg(false, "WASM tests skipped (setup failed)")
		} else {
			execArg := "wasmbrowsertest -quiet"
			testArgs := []string{"test", "-exec", execArg, "-cover", "./..."}
			execArg = "wasmbrowsertest"
			testArgs = []string{"test", "-exec", execArg, "-v", "-cover", "./..."}

			wasmCmd := exec.Command("go", testArgs...)
			wasmCmd.Env = os.Environ()
			wasmCmd.Env = append(wasmCmd.Env, "GOOS=js", "GOARCH=wasm")

			var wasmOut bytes.Buffer

			var wasmFilterCallback func(string)

			wasmFilter := NewConsoleFilter(wasmFilterCallback)
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
				// WASM test failure - ConsoleFilter already filtered the output in quiet mode
				addMsg(false, "tests wasm failed")
				testStatus = "Failed"
			} else {
				addMsg(true, "tests wasm ok")
				if testStatus != "Failed" {
					testStatus = "Passing"
				}
				wCov := calculateAverageCoverage(wOutput)
				if wCov != "0" {
					// Prefer WASM coverage if stdlib had 0% (common in WASM-only packages)
					if coveragePercent == "0" {
						coveragePercent = wCov
						addMsg(true, "coverage: "+coveragePercent+"%")
					}
				}
			}
		}
	}

	// Badges

	licenseType := "MIT"
	if checkFileExists("LICENSE") {
		// naive check
	}
	goVer := getGoVersion()

	bh := NewBadges()
	bh.SetLog(g.log)
	if err := bh.updateBadges("README.md", licenseType, goVer, testStatus, coveragePercent, raceStatus, vetStatus, true); err != nil {

	}

	// Return error if tests or vet failed
	summary := strings.Join(msgs, ", ")
	if testStatus == "Failed" || vetStatus == "Issues" {
		return summary, fmt.Errorf("%s", summary)
	}

	return summary, nil
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

func (g *Go) installWasmBrowserTest() error {
	if _, err := RunCommandSilent("which", "wasmbrowsertest"); err == nil {
		return nil
	}

	_, err := RunCommand("go", "install", "github.com/tinywasm/wasmbrowsertest@latest")
	if err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}
	return nil
}
