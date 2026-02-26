package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/tinywasm/devflow"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			runInit()
			return
		case "done":
			runDone()
			return
		default:
			printHelp()
			return
		}
	}

	env := devflow.NewDotEnv(".env")
	if val, ok := env.Get("CODEJOB"); ok {
		runQueryState(env, val)
		return
	}

	runDispatch(devflow.DefaultIssuePromptPath)
}

func runQueryState(env *devflow.DotEnv, val string) {
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Error: invalid CODEJOB value in .env:", val)
		return
	}
	driverName := parts[0]
	sessionID := parts[1]

	if driverName != "jules" {
		fmt.Fprintln(os.Stderr, "Error: unsupported driver in .env:", driverName)
		return
	}

	auth, _ := devflow.NewJulesAuth()
	apiKey, err := auth.EnsureAPIKey()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return
	}

	msg, prURL, done, err := devflow.JulesSessionState(sessionID, apiKey, &http.Client{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return
	}

	fmt.Println(msg)

	if done {
		git, _ := devflow.NewGit()
		if err := devflow.HandleDone(env, git, prURL); err != nil {
			fmt.Fprintln(os.Stderr, "Cleanup error:", err)
		}
	}
}

func runDispatch(path string) {
	job := devflow.NewCodeJob(devflow.NewJulesDriver(devflow.JulesConfig{}))
	if git, err := devflow.NewGit(); err == nil {
		job.SetRepoSync(git)
	}
	result, err := job.Send(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func runInit() {
	auth, err := devflow.NewJulesAuth()
	if err == nil && auth.HasKey() {
		fmt.Println("Already initialized (Jules API key found in keyring).")
		return
	}
	if err := devflow.NewCodeJobInitWizard().Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func runDone() {
	if err := devflow.MergePR(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Println("✅ PR merged, branch, docs/CHECK_PLAN.md removed")
}

func printHelp() {
	help := `CodeJob — Send coding tasks to external AI agents

Usage:
  codejob              Dispatch task from docs/PLAN.md
  codejob init         Setup: save Jules API key to system keyring
  codejob done         Close the loop: merge PR and cleanup

Examples:
  codejob              # dispatch default plan
  codejob init         # interactive setup wizard
  codejob done         # merge PR after review

Docs:
  https://github.com/tinywasm/devflow/blob/main/docs/CODEJOB.md
`
	fmt.Print(help)
}
