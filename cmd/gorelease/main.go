package main

import (
	"fmt"
	"os"

	"github.com/tinywasm/devflow"
)

func main() {
	usage := func() {
		fmt.Fprintf(os.Stderr, `gorelease - Create GitHub Release with cross-platform binaries

Usage:
    gorelease [tag]

Arguments:
    tag        Tag name (optional, uses latest tag if not provided)

Publication:
    If the current repository is PRIVATE, gorelease automatically attempts to
    publish the release to a PUBLIC repository named after the current directory.
    E.g. from ~/Dev/tinywasm/app with origin PRIVATE: publishes to <owner>/app.

Features:
    - Automatic target resolution (private source -> public distribution).
    - SHA256 checksums generation and upload.
    - Version injection (main.Version) and binary optimization (-s -w -trimpath).

Examples:
    gorelease
    gorelease v1.2.3

`)
	}

	tag, isHelp := devflow.ParseReleaseArgs(os.Args)

	if isHelp {
		usage()
		os.Exit(0)
	}

	git, err := devflow.NewGit()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	auth := devflow.NewGitHubOAuth()
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

	if err := goHandler.ReleaseOnly(tag, gh); err != nil {
		fmt.Println("Release failed:", err)
		os.Exit(1)
	}
}
