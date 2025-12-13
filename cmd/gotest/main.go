package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tinywasm/devflow"
)

func main() {
	fs := flag.NewFlagSet("gotest", flag.ExitOnError)
	verboseFlag := fs.Bool("v", false, "Enable verbose output")

	err := fs.Parse(os.Args[1:])
	if err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	git := devflow.NewGit()
	goHandler := devflow.NewGo(git)

	// Set logging if verbose
	if *verboseFlag {
		goHandler.SetLog(func(args ...any) {
			fmt.Println(args...)
		})
	}

	summary, err := goHandler.Test(*verboseFlag)
	if err != nil {
		fmt.Println("Tests failed:", err)
		os.Exit(1)
	}

	fmt.Println(summary)
}
