package devflow

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/tinywasm/context"
	"github.com/tinywasm/wizard"
	"golang.org/x/term"
)

// DefaultIssuePromptPath is the conventional location for the task description file.
const DefaultIssuePromptPath = "docs/PLAN.md"

// CodeJob orchestrates sending a coding task to a chain of AI agent drivers.
// It validates the prompt file, then tries each driver in priority order,
// falling back to the next on failure.
type CodeJob struct {
	drivers   []CodeJobDriver
	log       func(...any)
	publisher Publisher
}

// NewCodeJob creates a CodeJob with the given ordered drivers.
func NewCodeJob(drivers ...CodeJobDriver) *CodeJob {
	return &CodeJob{
		drivers: drivers,
		log:     func(...any) {},
	}
}

// SetLog sets the logging function for the orchestrator.
func (c *CodeJob) SetLog(fn func(...any)) {
	if fn != nil {
		c.log = fn
	}
}


// SetPublisher injects a Publisher for close-loop operations.
func (c *CodeJob) SetPublisher(p Publisher) { c.publisher = p }

// Run implements the unified API logic.
func (c *CodeJob) Run(message, tag string) (string, error) {
	env := NewDotEnv(".env")

	// 1. If message provided -> close the loop
	if message != "" {
		if _, ok := env.Get("CODEJOB_PR"); !ok {
			return "", fmt.Errorf("no pending PR found in .env (CODEJOB_PR missing)")
		}
		if c.publisher == nil {
			return "", fmt.Errorf("no publisher configured")
		}
		res, err := MergeAndPublish(c.publisher, message, tag)
		if err != nil {
			return "", err
		}

		if res.Tag == "RE_DISPATCH" {
			fmt.Println(res.Summary)
			return c.Send(DefaultIssuePromptPath)
		}

		return res.Summary, nil
	}

	// 2. No message -> check status or dispatch
	if val, ok := env.Get("CODEJOB"); ok {
		return c.checkStatus(env, val)
	}

	// 3. Auto-merge pending PR before dispatching new work
	if prURL, ok := env.Get("CODEJOB_PR"); ok && prURL != "" {
		if c.publisher == nil {
			return "", fmt.Errorf("no publisher configured")
		}
		res, err := MergeAndPublish(c.publisher, "chore: merge agent PR", "")
		if err != nil {
			return "", err
		}
		if res.Tag == "RE_DISPATCH" {
			fmt.Println(res.Summary)
			return c.Send(DefaultIssuePromptPath)
		}
		return res.Summary, nil
	}

	// 4. Auto-setup if API key missing
	auth, err := NewJulesAuth()
	if err == nil && !auth.HasKey() {
		if err := c.runSetupWizard(); err != nil {
			return "", err
		}
	}

	// 5. Dispatch
	return c.Send(DefaultIssuePromptPath)
}

func (c *CodeJob) checkStatus(env *DotEnv, val string) (string, error) {
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid CODEJOB value in .env: %s", val)
	}
	driverName := parts[0]
	sessionID := parts[1]

	if driverName != "jules" {
		return "", fmt.Errorf("unsupported driver in .env: %s", driverName)
	}

	auth, _ := NewJulesAuth()
	apiKey, err := auth.EnsureAPIKey()
	if err != nil {
		return "", err
	}

	msg, prURL, done, err := JulesSessionState(sessionID, apiKey, &http.Client{})
	if err != nil {
		return "", err
	}

	if done {
		git, _ := NewGit()
		if err := HandleDone(env, git, prURL); err != nil {
			return msg, fmt.Errorf("cleanup error: %w", err)
		}
	}

	return msg, nil
}

func (c *CodeJob) runSetupWizard() error {
	wiz := wizard.New(func(_ *context.Context) {
		fmt.Println("\n✅ Jules API key saved. Run 'codejob' to dispatch a task.")
	}, c)

	for wiz.WaitingForUser() {
		label := wiz.Label()
		fmt.Fprintf(os.Stderr, "\n%s: ", label)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("could not read API key: %w", err)
		}
		wiz.Change(string(raw))
		if !wiz.WaitingForUser() {
			break
		}
	}
	return nil
}

// GetSteps for wizard
func (c *CodeJob) GetSteps() []*wizard.Step {
	return []*wizard.Step{
		{
			LabelText: "Jules API Key (get yours at " + termLink(julesAPIKeyURL, julesAPIKeyURL) + ")",
			DefaultFn: func(ctx *context.Context) string { return "" },
			OnInputFn: func(in string, ctx *context.Context) (bool, error) {
				in = strings.TrimSpace(in)
				if in == "" {
					return false, fmt.Errorf("API key cannot be empty")
				}
				kr, err := NewKeyring()
				if err != nil {
					return false, fmt.Errorf("could not initialize keyring: %w", err)
				}
				if err := kr.Set(julesAPIKeyKey, in); err != nil {
					c.log(fmt.Sprintf("warning: could not save API key to keyring: %v", err))
				}
				return true, nil
			},
		},
	}
}

// Send validates issuePromptPath, publishes pending changes, then tries each
// driver in order until one succeeds. Returns an error if the file is missing,
// empty, the publish fails, or all drivers fail.
func (c *CodeJob) Send(issuePromptPath string) (string, error) {
	info, err := os.Stat(issuePromptPath)
	if err != nil {
		return "", fmt.Errorf("prompt file not found: %w", err)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("prompt file is empty: %s", issuePromptPath)
	}

	// PUBLISH BEFORE SEND (Stage 2, step 2.3)
	if c.publisher != nil {
		_, err := c.publisher.Publish("chore: sync before codejob dispatch", "", true, true, true, true, true)
		if err != nil {
			return "", fmt.Errorf("failed to sync repo before dispatch: %w", err)
		}
	}

	if len(c.drivers) == 0 {
		return "", fmt.Errorf("no drivers configured")
	}

	prompt := "Execute the implementation plan described in " + issuePromptPath
	title := autoDetectTitle()

	var lastErr error
	for _, d := range c.drivers {
		d.SetLog(c.log)
		result, err := d.Send(prompt, title)
		if err == nil {
			// Try to persist session ID to .env
			if sp, ok := d.(SessionProvider); ok {
				if id := sp.SessionID(); id != "" {
					env := NewDotEnv(".env")
					_ = env.Set("CODEJOB", strings.ToLower(d.Name())+":"+id)
				}
			}
			return result, nil
		}
		c.log(fmt.Sprintf("driver %s failed: %v", d.Name(), err))
		lastErr = err
	}

	return "", fmt.Errorf("all agents failed, last error: %w", lastErr)
}

// autoDetectTitle returns "owner/repo" for the current git repository,
// or "" if detection fails (non-fatal; driver will use its own fallback).
func autoDetectTitle() string {
	owner, repo, err := autoDetectOwnerRepo()
	if err != nil {
		return ""
	}
	return owner + "/" + repo
}
