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

	err := fs.Parse(os.Args[1:])
	if err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	git := devflow.NewGit()
	goHandler := devflow.NewGo(git)

	// Set logging if verbose
	if *verboseFlag {
		git.SetLog(func(args ...any) {
			fmt.Println(args...)
		})
		goHandler.SetLog(func(args ...any) {
			fmt.Println(args...)
		})
	}

	summary, err := goHandler.Push(*messageFlag, *tagFlag, *skipTestsFlag, *skipRaceFlag, *searchPathFlag)
	if err != nil {
		fmt.Println("Push failed:", err)
		os.Exit(1)
	}

	fmt.Println(summary)
}
