package devflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Test executes the test suite for the project.
// timeoutSec sets the per-package timeout in seconds (0 = default 30s).
func (g *Go) Test(customArgs []string, skipRace bool, timeoutSec int, noCache bool, runAll bool) (string, error) {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	hasCustomArgs := len(customArgs) > 0

	// Detect Module Name
	moduleName, err := getModuleName(".")
	if err != nil {
		return "", fmt.Errorf("error: %v", err)
	}

	// Check cache only for full suite runs
	if !hasCustomArgs && !noCache {
		cache := NewTestCache()
		if cache.IsCacheValid() {
			return cache.GetCachedMessage(), nil
		}
	}

	// Branch based on whether custom args are provided
	if hasCustomArgs {
		return g.runCustomTests(customArgs, moduleName, timeoutSec, runAll)
	}

	// Full test suite (run all phases)
	return g.runFullTestSuite(moduleName, skipRace, timeoutSec, noCache, runAll)
}

// runFullTestSuite executes the complete test suite (vet, race, cover, wasm, badges)
func (g *Go) runFullTestSuite(moduleName string, skipRace bool, timeoutSec int, noCache bool, runAll bool) (string, error) {
	// Check cache - if code hasn't changed since last successful test, return cached result
	if !noCache {
		cache := NewTestCache()
		if cache.IsCacheValid() {
			return cache.GetCachedMessage(), nil
		}
	}

	start := time.Now()

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
		vetArgs := []string{"vet"}
		if runAll {
			vetArgs = append(vetArgs, "-tags=integration")
		}
		vetArgs = append(vetArgs, "./...")
		vetOutput, vetErr = RunCommand("go", vetArgs...)
	}()

	// Check for WASM test files (async)
	go func() {
		defer wg1.Done()
		// Check for WASM test files
		// We do NOT return early for runAll anymore, we scan to see if actual WASM files exist.

		// 1. Get native test files
		nativeArgs := []string{"list", "-f", "{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}"}
		if runAll {
			nativeArgs = append(nativeArgs, "-tags=integration")
		}
		nativeArgs = append(nativeArgs, "./...")
		nativeCmd := exec.Command("go", nativeArgs...)
		nativeOut, _ := nativeCmd.CombinedOutput()

		// 2. Get WASM test files
		wasmArgs := []string{"list", "-f", "{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}"}
		if runAll {
			wasmArgs = append(wasmArgs, "-tags=integration")
		}
		wasmArgs = append(wasmArgs, "./...")
		wasmCmd := exec.Command("go", wasmArgs...)
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

	// Run tests with coverage and optional race detection
	// go test ./... automatically discovers all packages with tests
	var testErr error
	var testOutput string

	timeoutFlag := fmt.Sprintf("-timeout=%ds", timeoutSec)
	testArgs := []string{"test", "-v", "-cover", "-coverpkg=./...", "-count=1", timeoutFlag}

	if runAll {
		testArgs = append(testArgs, "-tags=integration")
	}

	testArgs = append(testArgs, "./...")
	if !skipRace {
		testArgs = append(testArgs[:1], append([]string{"-race"}, testArgs[1:]...)...)
	}

	// Safety net: context kills process 10s after Go's -timeout should have fired
	// This ensures we get the nice panic output from Go if possible
	testCtx, testCancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec+10)*time.Second)
	defer testCancel()
	testCmd := testCommand(testCtx, "go", testArgs...)

	testBuffer := &bytes.Buffer{}

	testFilter := NewConsoleFilter(g.consoleOutput)

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

	// Detect process-level timeout (killed by context)
	if testCtx.Err() == context.DeadlineExceeded {
		timedOut := findTimedOutTests(testOutput)
		if len(timedOut) > 0 {
			for _, name := range timedOut {
				addMsg(false, fmt.Sprintf("timeout: %s (exceeded %ds)", name, timeoutSec))
			}
		} else {
			addMsg(false, fmt.Sprintf("timeout: tests exceeded %ds", timeoutSec))
		}
		testStatus = "Failed"
	}

	// Process test results
	var stdTestsRan bool
	testStatus, raceStatus, stdTestsRan, msgs = evaluateTestResults(testErr, testOutput, moduleName, msgs, skipRace)

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
	// Process coverage results (from the same test run)
	if stdTestsRan {
		// Try exact coverage first (runs additional commands but provides accurate result)
		// We pass the same args used for testing
		if exactCov, err := g.getExactCoverage(testArgs); err == nil {
			coveragePercent = exactCov
		} else {
			// Fallback to average parsed from output
			coveragePercent = calculateAverageCoverage(testOutput)
		}
	}

	// WASM Tests
	var wasmTestOutput string
	if enableWasmTests {

		if err := g.installWasmBrowserTest(); err != nil {

			addMsg(false, "WASM tests skipped (setup failed)")
		} else {
			execArg := "wasmbrowsertest"
			// Add -count=1 to force cache bypass for WASM tests, consistent with native run
			testArgs := []string{"test", "-exec", execArg, "-v", "-cover", "-coverpkg=./...", "-count=1", "./..."}

			// Add cushion for WASM tests too
			wasmCtx, wasmCancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec+10)*time.Second)
			defer wasmCancel()
			wasmCmd := testCommand(wasmCtx, "go", testArgs...)
			wasmCmd.Env = os.Environ()
			wasmCmd.Env = append(wasmCmd.Env, "GOOS=js", "GOARCH=wasm")

			var wasmOut bytes.Buffer

			wasmFilter := NewConsoleFilter(g.consoleOutput)
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
			wasmTestOutput = wOutput

			// Detect process-level timeout for WASM tests
			if wasmCtx.Err() == context.DeadlineExceeded {
				timedOut := findTimedOutTests(wOutput)
				if len(timedOut) == 0 {
					// wasmbrowsertest buffers output: retry individually to find culprit
					timedOut = g.findWasmTimeoutCulprit(timeoutSec)
				}
				if len(timedOut) > 0 {
					for _, name := range timedOut {
						addMsg(false, fmt.Sprintf("timeout: %s (exceeded %ds)", name, timeoutSec))
					}
				} else {
					addMsg(false, fmt.Sprintf("timeout: wasm tests exceeded %ds", timeoutSec))
				}
				testStatus = "Failed"
			} else if err != nil {
				// WASM test failure - ConsoleFilter already filtered the output in quiet mode
				addMsg(false, "tests wasm failed")
				testStatus = "Failed"
			} else {
				addMsg(true, "tests wasm ok")
				if testStatus != "Failed" {
					testStatus = "Passing"
				}
				wCov := calculateAverageCoverage(wOutput)

				// Try exact coverage for WASM if possible (might need special handling for WASM env)
				// WASM tests are tricky because we use -exec wasmbrowsertest.
				// getExactCoverage can support it if we pass correct args.
				// But getExactCoverage implementation uses 'go test' which should respect GOOS/GOARCH from env.
				// Let's rely on calculateAverageCoverage for WASM for now unless we update getExactCoverage to support WASM env injection passed from here.
				// Actually, we can try getExactCoverage but we need to set Env.
				// For now, let's stick to parsing for WASM as it seems reliable (89.0 vs 89.0 from manual run was parsed correctly from go tool cover output in manual run)
				// Wait, manual run output "total: ... 89.0%".
				// The parsed output of `go test` usually doesn't show "total:" key unless using -coverprofile?
				// The output we parse is "coverage: 80.5% of statements".
				// So manual run showed 89.0% because I ran `go tool cover`.
				// `gotest` parsing only sees what `go test` emits.
				// If we want 89.0% here, we need getExactCoverage for WASM too.

				// Let's stick to simple parsing for WASM for now to avoid complexity with wasmbrowsertest + profile generation multiple times.
				// The user sees 76.7% vs 89% discrepancy mostly because Native tests were averaging 22 and 80.

				if wCov != "0" {
					wVal, _ := strconv.ParseFloat(wCov, 64)
					nVal, _ := strconv.ParseFloat(coveragePercent, 64)
					if wVal > nVal {
						coveragePercent = wCov
					}
				}
			}
		}
	}

	// Report consolidated coverage
	if coveragePercent != "0" {
		addMsg(true, "coverage: "+coveragePercent+"%")
	}

	// Detect slowest test across stdlib and WASM outputs
	allTestOutput := testOutput + "\n" + wasmTestOutput
	if name, dur := findSlowestTest(allTestOutput, 2.0); name != "" {
		msgs = append(msgs, fmt.Sprintf("⚠️ slow: %s (%.1fs)", name, dur))
	}

	// Detect timed out tests
	if timedOut := findTimedOutTests(allTestOutput); len(timedOut) > 0 {
		for _, name := range timedOut {
			addMsg(false, fmt.Sprintf("timeout: %s (exceeded %ds)", name, timeoutSec))
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
	elapsed := time.Since(start).Seconds()
	summary := fmt.Sprintf("%s (%.1fs)", strings.Join(msgs, ", "), elapsed)
	if testStatus == "Failed" || vetStatus == "Issues" {
		return summary, fmt.Errorf("%s", summary)
	}

	// Save test cache on success (for gopush optimization)
	// We save even if noCache=true, because this was a valid run
	cache := NewTestCache()
	if err := cache.SaveCache(summary); err != nil {
		g.log("Warning: failed to save test cache:", err)
	}

	return summary, nil
}

// runCustomTests executes tests with custom go test flags (fast path)
// Skips vet, badges, and cache, but runs WASM tests if detected
func (g *Go) runCustomTests(customArgs []string, moduleName string, timeoutSec int, runAll bool) (string, error) {
	start := time.Now()
	var msgs []string
	addMsg := func(ok bool, msg string) {
		symbol := "✅"
		if !ok {
			symbol = "❌"
		}
		msgs = append(msgs, fmt.Sprintf("%s %s", symbol, msg))
	}

	// Detect WASM tests in parallel with stdlib tests preparation
	var wg sync.WaitGroup
	var enableWasmTests bool

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Check for WASM test files by comparing native vs WASM test file lists
		// if runAll is set, we still check existence but include integration tags in detection

		nativeArgs := []string{"list", "-f", "{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}"}
		if runAll {
			nativeArgs = append(nativeArgs, "-tags=integration")
		}
		nativeArgs = append(nativeArgs, "./...")
		nativeCmd := exec.Command("go", nativeArgs...)
		nativeOut, _ := nativeCmd.CombinedOutput()

		wasmArgs := []string{"list", "-f", "{{.ImportPath}} {{.TestGoFiles}} {{.XTestGoFiles}}"}
		if runAll {
			wasmArgs = append(wasmArgs, "-tags=integration")
		}
		wasmArgs = append(wasmArgs, "./...")
		wasmCmd := exec.Command("go", wasmArgs...)
		wasmCmd.Env = os.Environ()
		wasmCmd.Env = append(wasmCmd.Env, "GOOS=js", "GOARCH=wasm")
		wasmOut, _ := wasmCmd.CombinedOutput()

		enableWasmTests = shouldEnableWasm(string(nativeOut), string(wasmOut))
	}()

	// Inject timeout if user didn't already pass -timeout
	timeoutFlag := fmt.Sprintf("-timeout=%ds", timeoutSec)
	if !hasTimeoutFlag(customArgs) {
		customArgs = append(customArgs, timeoutFlag)
	}

	// Build command: go test <customArgs> ./...
	testArgs := append([]string{"test"}, customArgs...)
	if runAll {
		testArgs = append(testArgs, "-tags=integration")
	}
	testArgs = append(testArgs, "./...")

	customCtx, customCancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec+10)*time.Second)
	defer customCancel()
	testCmd := testCommand(customCtx, "go", testArgs...)
	testBuffer := &bytes.Buffer{}

	// CRITICAL: Keep ConsoleFilter for clean output
	testFilter := NewConsoleFilter(g.consoleOutput)
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
	testErr := testCmd.Run()
	testFilter.Flush()

	testOutput := testBuffer.String()

	// Detect process-level timeout
	if customCtx.Err() == context.DeadlineExceeded {
		timedOut := findTimedOutTests(testOutput)
		if len(timedOut) > 0 {
			for _, name := range timedOut {
				addMsg(false, fmt.Sprintf("timeout: %s (exceeded %ds)", name, timeoutSec))
			}
		} else {
			addMsg(false, fmt.Sprintf("timeout: tests exceeded %ds", timeoutSec))
		}
		// Will be caught by evaluateTestResults as a failure
	}

	// Wait for WASM detection to complete
	wg.Wait()

	// Process stdlib test results (without race detection reporting)
	testStatus, _, stdTestsRan, msgs := evaluateTestResults(testErr, testOutput, moduleName, msgs, false)

	// Initialize coveragePercent for custom runs (not calculated for stdlib in fast path usually, but we need it for comparison)
	coveragePercent := "0"

	// Process coverage if std tests ran
	if stdTestsRan {
		if exactCov, err := g.getExactCoverage(testArgs); err == nil {
			coveragePercent = exactCov
		} else {
			coveragePercent = calculateAverageCoverage(testOutput)
		}
	}

	// Remove "race detection ok" message since we're not forcing -race in custom args
	// (user can add -race explicitly if desired)
	var filteredMsgs []string
	for _, msg := range msgs {
		if !strings.Contains(msg, "race detection ok") {
			filteredMsgs = append(filteredMsgs, msg)
		}
	}
	msgs = filteredMsgs

	// If no stdlib tests ran but we see exclusions, consider enabling WASM
	if !stdTestsRan {
		isExclusionError := strings.Contains(testOutput, "matched no packages") ||
			strings.Contains(testOutput, "build constraints exclude all Go files")
		if isExclusionError {
			enableWasmTests = true
			g.log("No stdlib tests matched/run (possibly WASM-only module), attempting WASM tests...")
		}
	}

	// Run WASM tests with same custom args (excluding -race)
	if enableWasmTests {
		if err := g.installWasmBrowserTest(); err != nil {
			addMsg(false, "WASM tests skipped (setup failed)")
		} else {
			// Build WASM test command with custom args, filtering out -race (not supported in WASM)
			var wasmArgs []string
			for _, arg := range customArgs {
				if arg != "-race" {
					wasmArgs = append(wasmArgs, arg)
				}
			}
			// Inject timeout for WASM tests too
			if !hasTimeoutFlag(wasmArgs) {
				wasmArgs = append(wasmArgs, timeoutFlag)
			}

			// Always add -count=1 for WASM to enforce consistent behavior (no caching)
			// unless user already specified it.
			hasCount := false
			for _, arg := range wasmArgs {
				if strings.Contains(arg, "-count") {
					hasCount = true
					break
				}
			}
			if !hasCount {
				wasmArgs = append(wasmArgs, "-count=1")
			}

			wasmTestArgs := append([]string{"test", "-exec", "wasmbrowsertest"}, wasmArgs...)
			if runAll {
				wasmTestArgs = append(wasmTestArgs, "-tags=integration")
			}
			wasmTestArgs = append(wasmTestArgs, "./...")

			wasmCtx, wasmCancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec+10)*time.Second)
			defer wasmCancel()
			wasmCmd := testCommand(wasmCtx, "go", wasmTestArgs...)
			wasmCmd.Env = os.Environ()
			wasmCmd.Env = append(wasmCmd.Env, "GOOS=js", "GOARCH=wasm")

			var wasmOut bytes.Buffer
			wasmFilter := NewConsoleFilter(g.consoleOutput)
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

			if wasmCtx.Err() == context.DeadlineExceeded {
				wOutput := wasmOut.String()
				timedOut := findTimedOutTests(wOutput)
				if len(timedOut) == 0 {
					timedOut = g.findWasmTimeoutCulprit(timeoutSec)
				}
				if len(timedOut) > 0 {
					for _, name := range timedOut {
						addMsg(false, fmt.Sprintf("timeout: %s (exceeded %ds)", name, timeoutSec))
					}
				} else {
					addMsg(false, fmt.Sprintf("timeout: wasm tests exceeded %ds", timeoutSec))
				}
				testStatus = "Failed"
			} else if err != nil {
				addMsg(false, "tests wasm failed")
				testStatus = "Failed"
			} else {
				wOutput := wasmOut.String()
				wCov := calculateAverageCoverage(wOutput)
				if wCov != "0" {
					wVal, _ := strconv.ParseFloat(wCov, 64)
					nVal, _ := strconv.ParseFloat(coveragePercent, 64)
					if wVal > nVal {
						coveragePercent = wCov
					}
				}
			}
		}
	}

	// Report consolidated coverage if available (and not 0)
	if coveragePercent != "0" {
		addMsg(true, "coverage: "+coveragePercent+"%")
	}

	elapsed := time.Since(start).Seconds()
	summary := fmt.Sprintf("%s (%.1fs)", strings.Join(msgs, ", "), elapsed)
	if testStatus == "Failed" {
		return summary, fmt.Errorf("%s", summary)
	}

	// NO cache save, NO badges (as requested)
	return summary, nil
}

// testCommand creates an exec.Cmd with graceful timeout handling.
// On timeout: sends SIGINT first (lets the process flush output), then SIGKILL after 5s.
func testCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

type paramWriter struct {
	write func(p []byte) (n int, err error)
}

func (p *paramWriter) Write(b []byte) (n int, err error) {
	return p.write(b)
}

// findSlowestTest parses -v test output and returns the name and duration of the slowest individual test
// across all packages if it exceeds the specified threshold.
func findSlowestTest(output string, threshold float64) (string, float64) {
	// Parse individual test timing from -v output: --- PASS: TestName (2.00s)
	testRe := regexp.MustCompile(`--- (?:PASS|FAIL): (\S+) \((\d+(?:\.\d+)?)s\)`)
	var slowestName string
	var slowestTime float64

	for _, match := range testRe.FindAllStringSubmatch(output, -1) {
		t, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			continue
		}
		if t > slowestTime {
			slowestName = match[1]
			slowestTime = t
		}
	}

	if slowestTime >= threshold {
		return slowestName, slowestTime
	}
	return "", 0
}

func calculateAverageCoverage(output string) string {
	lines := strings.Split(output, "\n")

	// Map to store max coverage per package
	// If package name is unknown, use unique key to treat as separate
	pkgCoverage := make(map[string]float64)

	// Regex to parse: ok package_name time coverage: X% of statements [in target_package]
	// We want to group by the TARGET package if specified ("in target_package"),
	// otherwise by the test package.
	// Actually, if we use -coverpkg=./..., many tests cover the SAME target package.
	// We want the coverage OF the target package.
	// So if "in X" is present, use X. If not, use test package Y.

	// Regex for "coverage: X% of statements in PACKAGE"
	reWithPkg := regexp.MustCompile(`coverage:\s+(\d+(\.\d+)?)%\s+of\s+statements\s+in\s+(\S+)`)

	// Regex for simple "coverage: X%" (fallback)
	reSimple := regexp.MustCompile(`coverage:\s+(\d+(\.\d+)?)%`)

	for _, line := range lines {
		if strings.Contains(line, "[no test files]") {
			continue
		}

		// Try explicit target package first
		matchesPkg := reWithPkg.FindStringSubmatch(line)
		if len(matchesPkg) > 3 {
			val, _ := strconv.ParseFloat(matchesPkg[1], 64)
			pkg := matchesPkg[3]
			if val > pkgCoverage[pkg] {
				pkgCoverage[pkg] = val
			}
			continue
		}

		// Fallback to simple coverage (usually implies covering itself)
		// We need to find the package name from the "ok" line start if possible
		// Line format: "ok  package_name  time  coverage: ..."
		matchesSimple := reSimple.FindStringSubmatch(line)
		if len(matchesSimple) > 1 {
			val, _ := strconv.ParseFloat(matchesSimple[1], 64)

			// Try to extract package name from start of line
			fields := strings.Fields(line)
			pkg := ""
			if len(fields) >= 2 && fields[0] == "ok" {
				pkg = fields[1]
			} else {
				// If we can't determine package, use the line itself as unique key to avoid merging
				pkg = line
			}

			if val > pkgCoverage[pkg] {
				pkgCoverage[pkg] = val
			}
		}
	}

	if len(pkgCoverage) == 0 {
		return "0"
	}

	var total float64
	for _, val := range pkgCoverage {
		total += val
	}

	return fmt.Sprintf("%.1f", total/float64(len(pkgCoverage)))
}

// getExactCoverage attempts to calculate weighted coverage using go tool cover
func (g *Go) getExactCoverage(testArgs []string) (string, error) {
	cmd := exec.Command("go", "list", "./...")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	packages := strings.Fields(string(out))
	if len(packages) == 0 {
		return "", fmt.Errorf("no packages")
	}

	tmpDir, err := os.MkdirTemp("", "gotest-cov")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	mergedParams := []string{"mode: set"}

	ctx := context.Background()

	var commonArgs []string
	for _, arg := range testArgs {
		if arg != "./..." && !strings.HasPrefix(arg, "-coverprofile") {
			commonArgs = append(commonArgs, arg)
		}
	}

	for i, pkg := range packages {
		profilePath := fmt.Sprintf("%s/%d.out", tmpDir, i)

		pkgArgs := append([]string{}, commonArgs...)
		pkgArgs = append(pkgArgs, pkg)
		pkgArgs = append(pkgArgs, fmt.Sprintf("-coverprofile=%s", profilePath))

		execCmd := exec.CommandContext(ctx, "go", pkgArgs...)
		execCmd.Env = os.Environ()
		_ = execCmd.Run()

		content, err := os.ReadFile(profilePath)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			if len(lines) > 1 {
				mergedParams = append(mergedParams, lines[1:]...)
			}
		}
	}

	mergedPath := fmt.Sprintf("%s/merged.out", tmpDir)
	var finalLines []string
	for _, l := range mergedParams {
		if strings.TrimSpace(l) != "" {
			finalLines = append(finalLines, l)
		}
	}
	if len(finalLines) <= 1 {
		return "", fmt.Errorf("no coverage data collected")
	}

	if err := os.WriteFile(mergedPath, []byte(strings.Join(finalLines, "\n")), 0644); err != nil {
		return "", err
	}

	coverCmd := exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", mergedPath))
	coverOut, err := coverCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cover tool failed: %v", err)
	}

	lines := strings.Split(string(coverOut), "\n")
	for _, line := range lines {
		if strings.Contains(line, "total:") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				last := parts[len(parts)-1] // 89.0%
				return strings.TrimSuffix(last, "%"), nil
			}
		}
	}

	return "", fmt.Errorf("total not found")
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
	// fmt.Printf("DEBUG: shouldEnableWasm check starting\n")
	nativeFiles := parseGoListFiles(nativeOut)
	// fmt.Printf("DEBUG: shouldEnableWasm - Native files found: %d\n", len(nativeFiles))
	wasmFiles := parseGoListFiles(wasmOut)
	// fmt.Printf("DEBUG: shouldEnableWasm - WASM files found: %d\n", len(wasmFiles))

	// Activation condition: at least one test file in WASM that is NOT in Native
	// This means it has a //go:build wasm tag or similar.
	for f := range wasmFiles {
		if !nativeFiles[f] {
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
		// fmt.Printf("DEBUG: parse line: %q\n", line)
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
	// fmt.Printf("DEBUG: Found %d unique test files\n", len(fileMap))
	return fileMap
}

// evaluateTestResults analyzes the output of go test and decides the outcome
// This function is pure and can be easily tested.
func evaluateTestResults(err error, output, moduleName string, msgs []string, skipRace bool) (testStatus, raceStatus string, stdTestsRan bool, newMsgs []string) {
	testStatus = "Failed"
	raceStatus = "Detected"
	if skipRace {
		raceStatus = "Skipped"
	}

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
		if !skipRace {
			raceStatus = "Clean"
			addMsg(true, "race detection ok")
		} else {
			addMsg(true, "race detection skipped")
		}
		addMsg(true, "tests stdlib ok")
		stdTestsRan = true
		return
	}

	// It failed (exit code != 0). Is it a real test failure or just build constraints?
	// Check for real test failures: "--- FAIL"
	// Also check for "FAIL\t" but EXCLUDE "[setup failed]" if we have valid tests passing elsewhere
	hasRealFailures := strings.Contains(output, "--- FAIL")

	if !hasRealFailures {
		// Look for FAIL lines that are NOT setup failures
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if (strings.Contains(line, "FAIL\t") || strings.Contains(line, "FAIL  ")) &&
				!strings.Contains(line, "[setup failed]") {
				hasRealFailures = true
				break
			}
		}
	}

	// Check for build failures: "[build failed]" or similar
	hasBuildFailures := strings.Contains(output, "[build failed]")

	// Check for exclusion errors (can be explicit or part of setup failed)
	isExclusionError := strings.Contains(output, "matched no packages") ||
		strings.Contains(output, "build constraints exclude all Go files")

	// Special case: Setup failed due to build constraints but other tests PASSED
	if !hasRealFailures && !hasBuildFailures {
		if strings.Contains(output, "[setup failed]") && isExclusionError && hasStdOk {
			// This is the "Partial Success" scenario (client)
			// Treat as success
		} else if strings.Contains(output, "[setup failed]") {
			// Setup failed for other reasons (and no other success confirmed logic override)
			hasRealFailures = true
		}
	}

	if !hasRealFailures && !hasBuildFailures && (isExclusionError || hasStdOk) {
		// It's a "Partial Success" or "Exclusion Only"
		testStatus = "Passing"
		if !skipRace {
			raceStatus = "Clean"
			if stdTestsRan {
				addMsg(true, "race detection ok")
			}
		} else {
			if stdTestsRan {
				addMsg(true, "race detection skipped")
			}
		}

		if stdTestsRan {
			addMsg(true, "tests stdlib ok")
		}
	} else {
		// Real failure
		addMsg(false, fmt.Sprintf("Test errors found in %s", moduleName))
	}

	return
}

// hasTimeoutFlag checks if -timeout is already present in the args
func hasTimeoutFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-timeout" || strings.HasPrefix(arg, "-timeout=") ||
			arg == "-test.timeout" || strings.HasPrefix(arg, "-test.timeout=") {
			return true
		}
	}
	return false
}

// findTimedOutTests parses go test output and extracts test names that timed out.
// Handles two scenarios:
// 1. Go's native timeout: "panic: test timed out after Ns\n  running tests:\n    TestName (Ns)"
// 2. Process killed externally (context.WithTimeout): finds the last "=== RUN" without a matching "--- PASS/FAIL"
func findTimedOutTests(output string) []string {
	// Try Go's native timeout format: "running tests:" section
	if strings.Contains(output, "running tests:") {
		re := regexp.MustCompile(`(?m)^\s+(\S+)\s+\(\d+`)
		var names []string
		inRunning := false
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "running tests:") {
				inRunning = true
				continue
			}
			if inRunning {
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					names = append(names, matches[1])
				} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "goroutine") {
					continue
				} else {
					break
				}
			}
		}
		if len(names) > 0 {
			return names
		}
	}

	// Fallback: find the last "=== RUN" without a matching "--- PASS/FAIL"
	// Works when process is killed externally (context timeout, SIGKILL)
	var lastRun string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "=== RUN") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				lastRun = fields[2]
			}
		}
		if strings.Contains(line, "--- PASS:") || strings.Contains(line, "--- FAIL:") {
			lastRun = ""
		}
	}
	if lastRun != "" {
		return []string{lastRun}
	}

	return nil
}

// discoverWasmTestNames scans WASM test source files for func TestXxx declarations.
// Used as fallback when wasmbrowsertest doesn't relay === RUN lines before a timeout kill.
// findWasmTimeoutCulprit retries WASM tests individually to identify which test hangs.
// Called after a bulk WASM run times out (wasmbrowsertest buffers output, so we can't
// determine the culprit from the output buffer).
func (g *Go) findWasmTimeoutCulprit(timeoutSec int) []string {
	names := discoverWasmTestNames()
	if len(names) <= 1 {
		return names
	}

	g.log("Identifying timed out wasm test...")

	for _, name := range names {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		cmd := testCommand(ctx, "go", "test", "-exec", "wasmbrowsertest", "-run", "^"+name+"$", "-v", "./...")
		cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Run()
		timedOut := ctx.Err() == context.DeadlineExceeded
		cancel()
		if timedOut {
			return []string{name}
		}
	}
	return nil
}

func discoverWasmTestNames() []string {
	listCmd := exec.Command("go", "list", "-f",
		`{{range .TestGoFiles}}{{$.Dir}}/{{.}} {{end}}{{range .XTestGoFiles}}{{$.Dir}}/{{.}} {{end}}`,
		"./...")
	listCmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	out, err := listCmd.Output()
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`func (Test\w+)\(`)
	var names []string
	seen := make(map[string]bool)
	for _, path := range strings.Fields(string(out)) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			if !seen[m[1]] {
				names = append(names, m[1])
				seen[m[1]] = true
			}
		}
	}
	return names
}
