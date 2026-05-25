package main

import (
	"fmt"
	"os"

	"github.com/tinywasm/devflow"
)

func main() {
	usage := func() {
		fmt.Fprintf(os.Stderr, `gorelease - Publish Go module + create GitHub Release with cross-platform binaries

Usage:
    gorelease 'commit message' [tag]

Arguments:
    message    Commit message (required)
    tag        Tag name (optional, auto-generated if not provided)

Examples:
    gorelease 'feat: new feature'
    gorelease 'fix: bug' 'v1.2.3'

`)
	}

	message, tag, isHelp := devflow.ParseCLIArgs(os.Args)

	if isHelp {
		usage()
		os.Exit(0)
	}

	// Message is mandatory
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

	log := func(args ...any) { fmt.Println(args...) }
	goHandler.SetLog(log)
	goHandler.SetConsoleOutput(func(s string) { fmt.Println(s) })

	gh, err := devflow.NewGitHub(log)
	if err != nil {
		fmt.Println("GitHub error:", err)
		os.Exit(1)
	}

	if err := goHandler.Release(message, tag, gh); err != nil {
		fmt.Println("Release failed:", err)
		os.Exit(1)
	}
}
