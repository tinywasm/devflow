package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tinywasm/devflow"
)

func main() {
	msg, tag, isHelp, isRelease, isResetGHToken := devflow.ParseCodeJobArgs(os.Args)
	if isHelp {
		showHelp()
		return
	}

	if isResetGHToken {
		auth, err := devflow.NewGitHubPATAuth()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		if err := auth.Reset(); err != nil {
			fmt.Fprintln(os.Stderr, "Error resetting GitHub token:", err)
			os.Exit(1)
		}
		fmt.Println("GitHub token reset successfully.")
		return
	}

	if msg == "" && !devflow.IsEnvironmentValid(".env") {
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

	log := func(args ...any) { fmt.Println(args...) }
	goHandler.SetLog(log)
	goHandler.SetConsoleOutput(func(s string) { fmt.Println(s) })

	// Ensure gh session is valid before creating the GitHub handler
	// to prevent the interactive device flow from triggering early.
	if err := devflow.EnsureGHSession(); err != nil {
		fmt.Fprintln(os.Stderr, "GitHub session error:", err)
		os.Exit(1)
	}

	patAuth, _ := devflow.NewGitHubPATAuth()
	gh, err := devflow.NewGitHub(log, patAuth)
	if err != nil {
		fmt.Fprintln(os.Stderr, "GitHub error:", err)
		os.Exit(1)
	}

	job := devflow.NewCodeJob(devflow.NewJulesDriver(devflow.JulesConfig{}))
	job.SetLog(log)
	job.SetPublisher(goHandler)

	// Inject the release function if -release flag is used
	if isRelease {
		job.SetReleaser(func(releaseTag string) error {
			return goHandler.ReleaseOnly(releaseTag, gh)
		})
	}

	result, err := job.Run(msg, tag, isRelease)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Format output: "jules: <id>" -> "Agent Jules • Session: <id>"
	if strings.HasPrefix(result, devflow.JulesResultPrefix) {
		sessionID := strings.TrimPrefix(result, devflow.JulesResultPrefix)
		fmt.Printf("Agent Jules • Session: %s\n", sessionID)
	} else {
		fmt.Println(result)
	}
}

func showHelp() {
	fmt.Println("Usage: codejob [message] [tag] [--reset-gh-token]")
	fmt.Println("\nArguments:")
	fmt.Println("  message            Commit message (optional, used when closing a loop)")
	fmt.Println("  tag                Explicit version tag (optional, e.g., v0.1.0)")
	fmt.Println("  --reset-gh-token   Remove the stored GitHub PAT from the keyring")
	fmt.Println("\nHelp Commands:")
	fmt.Println("  help, --help, -help, -h, h, ?, -?    Show this help message")
	fmt.Println("\nDescription:")
	fmt.Println("  CodeJob orchestrates coding tasks by sending instructions to AI agents.")
	fmt.Println("\nWorkflow:")
	fmt.Printf("  1. DISPATCH: Create %s and run 'codejob' to start a new task.\n", devflow.DefaultIssuePromptPath)
	fmt.Println("  2. REVIEW:   When ready, CodeJob renames PLAN.md to CHECK_PLAN.md and switches")
	fmt.Println("               automatically to the agent's branch for local review.")
	fmt.Println("  3. RESOLVE:")
	fmt.Println("     - APPROVE: Run 'codejob \"message\" [tag]' to merge the PR and publish.")
	fmt.Println("     - ITERATE: If adjustments are needed, create a new docs/PLAN.md and run")
	fmt.Println("                'codejob'. The old PR will be merged, CHECK_PLAN.md deleted,")
	fmt.Println("                and the new plan will be dispatched.")
}

