package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tinywasm/devflow"
)

func main() {
	msg, tag := parseArgs(os.Args)

	git, err := devflow.NewGit()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	goHandler, err := devflow.NewGo(git)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	job := devflow.NewCodeJob(devflow.NewJulesDriver(devflow.JulesConfig{}))
	job.SetLog(func(args ...any) { fmt.Println(args...) })
	job.SetPublisher(goHandler)

	result, err := job.Run(msg, tag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Format output: "jules: <id>" -> "Agent Jules • Session: <id>"
	if strings.HasPrefix(result, "jules: ") {
		sessionID := strings.TrimPrefix(result, "jules: ")
		fmt.Printf("Agent Jules • Session: %s\n", sessionID)
	} else {
		fmt.Println(result)
	}
}

func parseArgs(args []string) (message, tag string) {
	if len(args) > 1 {
		message = args[1]
	}
	if len(args) > 2 {
		tag = args[2]
	}
	return
}
