package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tinywasm/devflow"
)

func main() {
	usage := func() {
		fmt.Println("Usage: gotest [-t seconds] [go test flags]")
		fmt.Println()
		fmt.Println("No args: Full test suite (vet, race, cover, wasm, badges)")
		fmt.Println("With args: Pass flags to 'go test' (no vet/wasm/badges/cache)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -t N    Per-package timeout in seconds (default: 30)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  gotest              # Full suite, 30s timeout")
		fmt.Println("  gotest -t 120       # Full suite, 120s timeout")
		fmt.Println("  gotest -run TestFoo # Run specific test, 30s timeout")
		fmt.Println("  gotest -bench .     # Run benchmarks")
	}

	// Handle help requests
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "?" || arg == "help" || arg == "-h" || arg == "--help" {
			usage()
			os.Exit(0)
		}
	}

	// Extract -t N (gotest-specific timeout flag) from args
	timeoutSec := 30
	var customArgs []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-t" && i+1 < len(args) {
			if v, err := strconv.Atoi(args[i+1]); err == nil && v > 0 {
				timeoutSec = v
			}
			i++ // skip value
		} else {
			customArgs = append(customArgs, args[i])
		}
	}

	git, err := devflow.NewGit()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	goHandler, err := devflow.NewGo(git)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	summary, err := goHandler.Test(customArgs, false, timeoutSec)
	if err != nil {
		fmt.Println("Tests failed:", err)
		os.Exit(1)
	}

	fmt.Println(summary)
}
