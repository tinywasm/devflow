package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tinywasm/devflow"
)

func main() {
	fs := flag.NewFlagSet("gopush", flag.ExitOnError)
	messageFlag := fs.String("m", "", "Commit message")
	tagFlag := fs.String("t", "", "Tag")
	skipTestsFlag := fs.Bool("skip-tests", false, "Skip tests")
	skipRaceFlag := fs.Bool("skip-race", false, "Skip race tests")
	searchPathFlag := fs.String("search-path", "..", "Path to search for dependent modules")
	verboseFlag := fs.Bool("v", false, "Enable verbose output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `gopush - Complete Go project workflow: test + git push + update dependents

Usage:
    gopush 'commit message' [tag] [options]
    gopush -m 'commit message' [options]

Arguments:
    message    Commit message (required)
    tag        Tag name (optional, auto-generated if not provided)

Options:
    -m              Commit message
    -t              Tag
    --skip-tests    Skip all tests
    --skip-race     Skip race tests
    --search-path   Path to search for dependent modules (default: "..")
    -v              Enable verbose output
    -h, --help      Show this help message

Examples:
    gopush 'feat: new feature'
    gopush 'fix: bug' 'v1.2.3'
    gopush -m 'chore: docs' --skip-tests

`)
	}

	err := fs.Parse(os.Args[1:])
	if err != nil {
		// flag.ExitOnError will handle this, but just in case
		os.Exit(1)
	}

	// Check for help flag
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fs.Usage()
			os.Exit(0)
		}
	}

	// Get message from flag or positional argument
	message := *messageFlag
	tag := *tagFlag
	args := fs.Args()

	// If no -m flag, try to get message from positional arguments
	if message == "" && len(args) > 0 {
		message = args[0]
		// If tag not set via -t flag, try second positional argument
		if tag == "" && len(args) > 1 {
			tag = args[1]
		}
	}

	if message == "" {
		fmt.Fprintln(os.Stderr, "Error: commit message is required")
		fs.Usage()
		os.Exit(1)
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

	// Set logging if verbose
	if *verboseFlag {
		git.SetLog(func(args ...any) {
			fmt.Println(args...)
		})
		goHandler.SetLog(func(args ...any) {
			fmt.Println(args...)
		})
	}

	summary, err := goHandler.Push(message, tag, *skipTestsFlag, *skipRaceFlag, *searchPathFlag)
	if err != nil {
		fmt.Println("Push failed:", err)
		os.Exit(1)
	}

	fmt.Println(summary)
}
