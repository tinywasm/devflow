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

	// Pre-process flags to keep positional args consistent
	var skipRace bool
	filteredArgs := []string{os.Args[0]}
	for _, arg := range os.Args[1:] {
		if arg == "--skip-race" || arg == "-R" {
			skipRace = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	message, tag, isHelp, _ := devflow.ParseCLIArgs(filteredArgs)

	if isHelp || (len(filteredArgs) == 1 && !devflow.IsEnvironmentValid(".env")) {
		usage()
		os.Exit(0)
	}

	// Message is mandatory if not in an active codejob session
	if message == "" && !devflow.IsEnvironmentValid(".env") {
		usage()
		os.Exit(0)
	}

	git, err := devflow.NewGit()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	auth := devflow.NewGitHubAuth()
	git.SetAuthRetrier(auth)

	goHandler, err := devflow.NewGo(git)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Run Push with parsed options
	summary, err := goHandler.Push(message, tag, false, skipRace, false, false, false, false, "..")
	if err != nil {
		fmt.Println("Push failed:", err)
		os.Exit(1)
	}

	fmt.Println(summary.Summary)
}
