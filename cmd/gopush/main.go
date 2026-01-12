package main

import (
	"fmt"
	"os"

	"github.com/tinywasm/devflow"
)

func main() {
	usage := func() {
		fmt.Fprintf(os.Stderr, `gopush - Complete Go project workflow: test + git push + update dependents

Usage:
    gopush 'commit message' [tag]

Arguments:
    message    Commit message (required)
    tag        Tag name (optional, auto-generated if not provided)

Examples:
    gopush 'feat: new feature'
    gopush 'fix: bug' 'v1.2.3'

`)
	}

	args := os.Args[1:]

	// Check if help requested or no arguments
	if len(args) == 0 {
		usage()
		os.Exit(0)
	}

	firstArg := args[0]
	if firstArg == "help" || firstArg == "?" || firstArg == "-h" || firstArg == "--help" {
		usage()
		os.Exit(0)
	}

	// Message is mandatory
	message := firstArg
	tag := ""
	if len(args) > 1 {
		tag = args[1]
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

	// Always run with defaults
	summary, err := goHandler.Push(message, tag, false, false, "..")
	if err != nil {
		fmt.Println("Push failed:", err)
		os.Exit(1)
	}

	fmt.Println(summary)
}
