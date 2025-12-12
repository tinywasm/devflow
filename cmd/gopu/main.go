package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cdvelop/gitgo"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `gopu - Automated Go & Git workflow

Usage:
    gopu "commit message" [module_path] [search_path]
    gopu [options]

Arguments:
    message      Commit message (required if no changes)
    module_path  Current module import path (e.g. github.com/user/repo)
    search_path  Path to search for dependent modules (default: ..)

Options:
    -h, --help     Show this help message

Workflow:
    1. go mod tidy
    2. git add .
    3. git commit -m "message"
    4. git tag <next_tag>
    5. git push && git push origin <tag>
    6. Update dependent modules (go get -u module@tag && go mod tidy)

`)
	}

	helpFlag := flag.Bool("h", false, "Show help")
	flag.BoolVar(helpFlag, "help", false, "Show help")
	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	args := flag.Args()

	var message, modulePath, searchPath string

	if len(args) > 0 {
		message = args[0]
	}

	if len(args) > 1 {
		modulePath = args[1]
	}

	if len(args) > 2 {
		searchPath = args[2]
	} else {
        searchPath = ".."
    }

	if err := gitgo.WorkflowGoPush(message, modulePath, searchPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

    os.Exit(0)
}
