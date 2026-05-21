package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tinywasm/devflow"
)

func main() {
	msg, tag, isHelp := parseArgs(os.Args)
	if isHelp {
		showHelp()
		return
	}

	if msg == "" && !isEnvironmentValid() {
		showHelp()
		return
	}

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

func showHelp() {
	fmt.Println("Usage: codejob [message] [tag]")
	fmt.Println("\nArguments:")
	fmt.Println("  message    Commit message (optional, used when closing a loop)")
	fmt.Println("  tag        Explicit version tag (optional, e.g., v0.1.0)")
	fmt.Println("\nHelp Commands:")
	fmt.Println("  help, --help, -help, -h, h, ?, -?    Show this help message")
	fmt.Println("\nDescription:")
	fmt.Println("  CodeJob orchestrates coding tasks by sending instructions to AI agents.")
	fmt.Println("\nWorkflow:")
	fmt.Println("  1. DISPATCH: Create docs/PLAN.md and run 'codejob' to start a new task.")
	fmt.Println("  2. REVIEW:   When ready, CodeJob renames PLAN.md to CHECK_PLAN.md and switches")
	fmt.Println("               automatically to the agent's branch for local review.")
	fmt.Println("  3. RESOLVE:")
	fmt.Println("     - APPROVE: Run 'codejob \"message\" [tag]' to merge the PR and publish.")
	fmt.Println("     - ITERATE: If adjustments are needed, create a new docs/PLAN.md and run")
	fmt.Println("                'codejob'. The old PR will be merged, CHECK_PLAN.md deleted,")
	fmt.Println("                and the new plan will be dispatched.")
}

func isEnvironmentValid() bool {
	// 1. Check process environment
	if os.Getenv("CODEJOB") != "" || os.Getenv("CODEJOB_PR") != "" {
		return true
	}

	// 2. Check .env file
	env := devflow.NewDotEnv(".env")
	if val, ok := env.Get("CODEJOB"); ok && val != "" {
		return true
	}
	if val, ok := env.Get("CODEJOB_PR"); ok && val != "" {
		return true
	}

	// 3. Check for pending PLAN.md
	if _, err := os.Stat(devflow.DefaultIssuePromptPath); err == nil {
		return true
	}

	return false
}

func parseArgs(args []string) (message, tag string, isHelp bool) {
	if len(args) > 1 {
		arg := strings.ToLower(args[1])
		switch arg {
		case "help", "-help", "--help", "h", "-h", "?", "-?":
			return "", "", true
		}
		message = args[1]
	}
	if len(args) > 2 {
		tag = args[2]
	}
	return
}
