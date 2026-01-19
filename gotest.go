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

	// Check for WASM test files by comparing native vs WASM test file lists (async)
	go func() {
		defer wg1.Done()

		// 1. Get native test files
		nativeCmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}", "./...")
		nativeOut, _ := nativeCmd.CombinedOutput()

		// 2. Get WASM test files
		wasmCmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}", "./...")
		wasmCmd.Env = os.Environ()
		wasmCmd.Env = append(wasmCmd.Env, "GOOS=js", "GOARCH=wasm")
		wasmOut, _ := wasmCmd.CombinedOutput()

		// 3. Decision logic
		enableWasmTests = shouldEnableWasm(string(nativeOut), string(wasmOut))
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
	var stdTestsRan bool
	testStatus, raceStatus, stdTestsRan, msgs = evaluateTestResults(testErr, testOutput, moduleName, msgs)

	// If no stdlib tests ran but we see exclusions, consider enabling WASM (if not already enabled)
	if !stdTestsRan {
		isExclusionError := strings.Contains(testOutput, "matched no packages") ||
			strings.Contains(testOutput, "build constraints exclude all Go files")
		if isExclusionError {
			enableWasmTests = true
			g.log("No stdlib tests matched/run (possibly WASM-only module), skipping stdlib tests...")
		}
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

// shouldEnableWasm decides if WASM tests should be run based on go list output differences
func shouldEnableWasm(nativeOut, wasmOut string) bool {
	fmt.Printf("DEBUG: shouldEnableWasm check starting\n")
	nativeFiles := parseGoListFiles(nativeOut)
	fmt.Printf("DEBUG: shouldEnableWasm - Native files found: %d\n", len(nativeFiles))
	wasmFiles := parseGoListFiles(wasmOut)
	fmt.Printf("DEBUG: shouldEnableWasm - WASM files found: %d\n", len(wasmFiles))

	// Activation condition: at least one test file in WASM that is NOT in Native
	// This means it has a //go:build wasm tag or similar.
	for f := range wasmFiles {
		if !nativeFiles[f] {
			fmt.Printf("WASM unique test file: %s\n", f)
			return true
		}
	}
	return false
}

// parseGoListFiles converts the output of go list into a map of unique test files
func parseGoListFiles(output string) map[string]bool {
	fileMap := make(map[string]bool)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fmt.Printf("DEBUG: parse line: %q\n", line)
		// Legitimate go list lines for this template usually contain '['
		// but we must skip error messages that might start with "package" or involve "syscall/js"
		if !strings.Contains(line, "[") {
			continue
		}

		// Extract package path and file list: "path [a_test.go b_test.go] []"
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		pkgPath := parts[0]
		fileList := parts[1]

		// Final check: pkgPath shouldn't have spaces if it's a real path from go list
		if strings.Contains(pkgPath, " ") {
			continue
		}

		// Normalize file list and add to map: pkgPath/file
		fileList = strings.ReplaceAll(fileList, "[", "")
		fileList = strings.ReplaceAll(fileList, "]", "")
		files := strings.Fields(fileList)
		for _, f := range files {
			fileMap[pkgPath+"/"+f] = true
		}
	}
	fmt.Printf("DEBUG: Found %d unique test files\n", len(fileMap))
	return fileMap
}

// evaluateTestResults analyzes the output of go test and decides the outcome
// This function is pure and can be easily tested.
func evaluateTestResults(err error, output, moduleName string, msgs []string) (testStatus, raceStatus string, stdTestsRan bool, newMsgs []string) {
	testStatus = "Failed"
	raceStatus = "Detected"
	newMsgs = msgs

	addMsg := func(ok bool, msg string) {
		symbol := "✅"
		if !ok {
			symbol = "❌"
		}
		newMsgs = append(newMsgs, fmt.Sprintf("%s %s", symbol, msg))
	}

	// Determine if any stdlib tests actually ran by looking for ok/FAIL markers in output
	// Use more robust matching that handles different spacing/tabs
	hasStdOk := strings.Contains(output, "ok  ") || strings.Contains(output, "ok\t") || strings.Contains(output, "\tok\t")
	hasStdFail := strings.Contains(output, "FAIL  ") || strings.Contains(output, "FAIL\t") || strings.Contains(output, "\tFAIL\t")
	stdTestsRan = hasStdOk || hasStdFail

	if err == nil {
		testStatus = "Passing"
		raceStatus = "Clean"
		addMsg(true, "tests stdlib ok")
		addMsg(true, "race detection ok")
		stdTestsRan = true
		return
	}

	// It failed (exit code != 0). Is it a real test failure or just build constraints?
	// Check for real test failures: "--- FAIL"
	hasRealFailures := strings.Contains(output, "--- FAIL") || strings.Contains(output, "\nFAIL\t")

	// Check for build failures: "[build failed]" or similar
	hasBuildFailures := strings.Contains(output, "[build failed]")

	// Check for exclusion errors
	isExclusionError := strings.Contains(output, "matched no packages") ||
		strings.Contains(output, "build constraints exclude all Go files")

	if !hasRealFailures && !hasBuildFailures && isExclusionError {
		// It's a "Partial Success" or "Exclusion Only"
		testStatus = "Passing"
		raceStatus = "Clean"
		if stdTestsRan {
			addMsg(true, "tests stdlib ok (some subpackages excluded)")
			addMsg(true, "race detection ok")
		}
	} else {
		// Real failure
		addMsg(false, fmt.Sprintf("Test errors found in %s", moduleName))
	}

	return
}
