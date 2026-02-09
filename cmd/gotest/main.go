package main

import (
	"fmt"
	"os"

	"github.com/tinywasm/devflow"
)

func main() {
	usage := func() {
		fmt.Println("Usage: gotest [go test flags]")
		fmt.Println()
		fmt.Println("No args: Full test suite (vet, race, cover, wasm, badges)")
		fmt.Println("With args: Pass flags to 'go test' (no vet/wasm/badges/cache)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  gotest              # Full suite")
		fmt.Println("  gotest -v           # Verbose output (filtered)")
		fmt.Println("  gotest -run TestFoo # Run specific test")
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

	// Forward all arguments to Test()
	customArgs := os.Args[1:]

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

	summary, err := goHandler.Test(customArgs, false)
	if err != nil {
		fmt.Println("Tests failed:", err)
		os.Exit(1)
	}

	fmt.Println(summary)
}
