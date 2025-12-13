package devflow

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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

	// Go Vet
	vetOutput, err := RunCommand("go", "vet", "./...")
	if err != nil {
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
			addMsg(true, "vet passed")
		}
	} else {
		vetStatus = "OK"
		addMsg(true, "vet passed")
	}

	// Check for test files
	hasTestFiles := false
	out, _ := RunCommand("find", ".", "-type", "f", "-name", "*_test.go")
	if len(out) > 0 {
		hasTestFiles = true
	}

	// Check if there are WASM-specific test files (auto-detect)
	enableWasmTests := false
	wasmTestOut, _ := RunCommand("find", ".", "-type", "f", "-name", "*Wasm*_test.go", "-o", "-name", "*wasm*_test.go")
	if len(wasmTestOut) > 0 {
		enableWasmTests = true
		if !quiet {
			g.log("Detected WASM test files, will run WASM tests...")
		}
	}

	if hasTestFiles {
		if !quiet {
			g.log("Running standard Go tests...")
		}

		testCmd := exec.Command("go", "test", "./...")

		// Setup ConsoleFilter
		filter := NewConsoleFilter(quiet, func(s string) {
			if quiet {
				fmt.Println(s)
			} else {
				fmt.Println(s)
			}
		})

		validTestOutput := &bytes.Buffer{}

		pipeWriter := &paramWriter{
			write: func(p []byte) (n int, err error) {
				s := string(p)
				validTestOutput.Write(p)
				filter.Add(s)
				return len(p), nil
			},
		}

		testCmd.Stdout = pipeWriter
		testCmd.Stderr = pipeWriter

		err := testCmd.Run()
		filter.Flush()

		output := validTestOutput.String()

		stdTestsRan := false
		if err != nil {
			if strings.Contains(output, "matched no packages") {
				testStatus = "Passing"
				if !enableWasmTests {
					enableWasmTests = true
					if !quiet {
						g.log("No standard tests found, auto-enabling WASM tests...")
					}
				}
			} else {
				addMsg(false, fmt.Sprintf("Test errors found in %s", moduleName))
				stdTestsRan = true
			}
		} else {
			testStatus = "Passing"
			addMsg(true, "tests stdlib passed")
			stdTestsRan = true
		}

		// Race Detection
		if stdTestsRan {
			if !quiet {
				g.log("Running race detection...")
			}
			raceCmd := exec.Command("go", "test", "-race", "./...")

			raceBuffer := &bytes.Buffer{}
			raceFilter := NewConsoleFilter(quiet, nil)

			racePipe := &paramWriter{
				write: func(p []byte) (n int, err error) {
					s := string(p)
					raceBuffer.Write(p)
					raceFilter.Add(s)
					return len(p), nil
				},
			}

			raceCmd.Stdout = racePipe
			raceCmd.Stderr = racePipe
			err = raceCmd.Run()
			raceFilter.Flush()

			rOutput := raceBuffer.String()

			if err != nil {
				if strings.Contains(rOutput, "matched no packages") {
					raceStatus = "Clean"
				} else {
					raceStatus = "Detected"
					addMsg(false, fmt.Sprintf("Race condition tests failed in %s", moduleName))
				}
			} else {
				raceStatus = "Clean"
				addMsg(true, "race detection passed")
			}
		} else {
			raceStatus = "Clean"
		}

		// Coverage
		if stdTestsRan {
			if !quiet {
				g.log("Calculating coverage...")
			}
			out, err := RunCommand("go", "test", "-cover", "./...")
			if err == nil {
				coveragePercent = calculateAverageCoverage(out)
				if coveragePercent != "0" {
					addMsg(true, "coverage: "+coveragePercent+"%")
				}
			} else if strings.Contains(out, "matched no packages") {
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
					addMsg(true, "tests wasm passed")
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
