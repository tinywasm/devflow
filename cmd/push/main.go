package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cdvelop/gitgo"
)

func main() {
	// Parse flags
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `push - Automated Git workflow

Usage:
    push "commit message" [tag]
    push [options]

Arguments:
    message    Commit message (required if no changes)
    tag        Tag name (optional, auto-generated if not provided)

Options:
    -h, --help     Show this help message

Examples:
    push "feat: new feature"
    push "fix: bug correction" "v1.2.3"

Workflow:
    1. git add .
    2. git commit -m "message"
    3. git tag <tag> (auto-generated or provided)
    4. git push && git push origin <tag>

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

	var message, tag string

	if len(args) > 0 {
		message = args[0]
	}

	if len(args) > 1 {
		tag = args[1]
	}

	// Execute workflow
	git := gitgo.NewGit()
	summary, err := git.Push(message, tag)

	if summary != "" {
		fmt.Println(summary)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
