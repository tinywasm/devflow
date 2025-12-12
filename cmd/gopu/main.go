package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cdvelop/gitgo"
)

func main() {
	// Flags
	helpFlag := flag.Bool("h", false, "Show help")
	flag.BoolVar(helpFlag, "help", false, "Show help")
	skipTestsFlag := flag.Bool("skip-tests", false, "Skip running tests")
	skipRaceFlag := flag.Bool("skip-race", false, "Skip race detector tests")
	skipUpdateFlag := flag.Bool("skip-update", false, "Skip updating dependent modules")
	searchPathFlag := flag.String("search", "..", "Path to search for dependent modules")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `gopu - Automated Go Project Update Workflow

Usage:
    gopu "commit message" [tag]
    gopu [options]

Arguments:
    message    Commit message
    tag        Tag name (optional, auto-generated if not provided)

Options:
    -h, --help         Show this help message
    --skip-tests       Skip running tests
    --skip-race        Skip race detector tests
    --skip-update      Skip updating dependent modules
    --search PATH      Path to search for dependent modules (default: "..")

Examples:
    gopu "feat: new feature"
    gopu "fix: bug" "v1.2.3"
    gopu --skip-race "quick fix"
    gopu --skip-update "docs only"

Workflow:
    1. go mod verify
    2. go test ./...
    3. go test -race ./...
    4. git add, commit, tag, push (via push workflow)
    5. Update dependent modules with new version
    6. go get -u module@version in dependents
    7. go mod tidy in dependents

`)
	}

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	// Positional arguments
	args := flag.Args()

	var message, tag string

	if len(args) > 0 {
		message = args[0]
	}

	if len(args) > 1 {
		tag = args[1]
	}

	// Determine if skip update
	skipUpdate := *skipUpdateFlag
	searchPath := *searchPathFlag
	if skipUpdate {
		searchPath = "" // Don't search if update is skipped
	}

	// Execute workflow
	err := gitgo.WorkflowGoPU(
		message,
		tag,
		*skipTestsFlag,
		*skipRaceFlag,
		searchPath,
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
