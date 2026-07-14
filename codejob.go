package devflow

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tinywasm/context"
	"github.com/tinywasm/wizard"
	"golang.org/x/term"
)

const (
	// DefaultIssuePromptPath is the conventional location for the task description file.
	DefaultIssuePromptPath = "docs/PLAN.md"

	// EnvKeyCodejob holds the active agent session ("driver:phase:ref").
	EnvKeyCodejob = "CODEJOB"
	// EnvKeyCodejobPR holds the GitHub PR URL pending merge.
	// Deprecated: legacy key, read-only for migration.
	EnvKeyCodejobPR = "CODEJOB_PR"

	// CodejobStashMessage is the label for the git stash created during branch switching.
	CodejobStashMessage = "codejob: local drift before review"

	// HintManualCheckout is the message shown when automatic branch switch fails.
	HintManualCheckout = "⚠️  Could not switch branch automatically — run manually:"
	// HintManualPRCheckout is the message shown when PR branch resolution fails.
	HintManualPRCheckout = "⚠️  Could not resolve branch from PR — switch manually:"
)

type CodejobPhase string

const (
	PhaseRunning CodejobPhase = "running" // agent working; Ref = session ID
	PhaseReview  CodejobPhase = "review"  // PR open, pending merge; Ref = PR URL
)

// CodejobState is the single piece of state the codejob manager persists.
type CodejobState struct {
	Driver string
	Phase  CodejobPhase
	Ref    string // session ID (running) or PR URL (review)
}

// ErrInvalidCodejobState is returned when the CODEJOB value in .env is malformed.
var ErrInvalidCodejobState = fmt.Errorf("invalid CODEJOB value in .env: expected <driver>:<phase>:<ref>")

// ParseCodejobState parses a raw CODEJOB value.
func ParseCodejobState(raw string) (CodejobState, error) {
	if raw == "" {
		return CodejobState{}, nil
	}
	parts := strings.SplitN(raw, ":", 3)
	if len(parts) == 2 {
		// Legacy format: driver:sessionID
		return CodejobState{Driver: parts[0], Phase: PhaseRunning, Ref: parts[1]}, nil
	}
	if len(parts) != 3 {
		return CodejobState{}, ErrInvalidCodejobState
	}
	phase := CodejobPhase(parts[1])
	if phase != PhaseRunning && phase != PhaseReview {
		return CodejobState{}, ErrInvalidCodejobState
	}
	return CodejobState{Driver: parts[0], Phase: phase, Ref: parts[2]}, nil
}

func (s CodejobState) String() string {
	if s.Driver == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", s.Driver, s.Phase, s.Ref)
}

// LoadCodejobState reads the codejob state from .env, with legacy migration.
func LoadCodejobState(env *DotEnv) (CodejobState, error) {
	// 1. Try legacy PR key first (it wins if present)
	if prURL, ok := env.Get(EnvKeyCodejobPR); ok && prURL != "" {
		return CodejobState{Driver: "jules", Phase: PhaseReview, Ref: prURL}, nil
	}

	// 2. Try unified CODEJOB key
	val, ok := env.Get(EnvKeyCodejob)
	if !ok || val == "" {
		return CodejobState{}, nil
	}

	return ParseCodejobState(val)
}

// SaveCodejobState writes the state to .env and removes legacy keys.
func SaveCodejobState(env *DotEnv, s CodejobState) error {
	if s.Driver == "" {
		return ClearCodejobState(env)
	}
	if err := env.Set(EnvKeyCodejob, s.String()); err != nil {
		return err
	}
	return env.Delete(EnvKeyCodejobPR)
}

// ClearCodejobState removes all codejob-related keys from .env.
func ClearCodejobState(env *DotEnv) error {
	if err := env.Delete(EnvKeyCodejob); err != nil {
		return err
	}
	return env.Delete(EnvKeyCodejobPR)
}

// CodeJob orchestrates sending a coding task to a chain of AI agent drivers.
// It validates the prompt file, then tries each driver in priority order,
// falling back to the next on failure.
type CodeJob struct {
	drivers   []CodeJobDriver
	log       func(...any)
	publisher Publisher
	releaseFn func(tag string) error
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


// ObjectsToPublish implements PublishObjector. It is stateless.
func (CodeJob) ObjectsToPublish(ctx PublishContext) (PublishAction, string) {
	// Skip if a session is active (running or review).
	// Running: agent is working, do not touch.
	// Review: local tree is on PR branch, deps updates would commit into the PR.
	phase := CodejobPhaseOf(ctx.RepoDir)
	if phase == PhaseRunning || phase == PhaseReview {
		return ActionSkip, ObjectionCodejobSession
	}
	if _, err := os.Stat(filepath.Join(ctx.RepoDir, DefaultIssuePromptPath)); err == nil {
		return ActionDepsOnly, ObjectionPlanPending
	}
	return ActionNone, ""
}

// SetPublisher injects a Publisher for close-loop operations.
func (c *CodeJob) SetPublisher(p Publisher) { c.publisher = p }

// SetReleaser injects a release function to be called after MergeAndPublish when -release flag is used.
func (c *CodeJob) SetReleaser(fn func(tag string) error) { c.releaseFn = fn }

// IsEnvironmentValid reports whether the current working directory has an
// active codejob context: a running session, a pending PR, or a PLAN.md to dispatch.
// dotenvPath is the path to the .env file (typically ".env").
func IsEnvironmentValid(dotenvPath string) bool {
	env := NewDotEnv(dotenvPath)
	state, _ := LoadCodejobState(env)
	if state.Phase != "" {
		return true
	}
	if _, err := os.Stat(DefaultIssuePromptPath); err == nil {
		return true
	}
	return false
}

// Run implements the unified API logic.
// isRelease indicates whether to create a GitHub Release after MergeAndPublish.
func (c *CodeJob) Run(message, tag string, isRelease bool) (string, error) {
	env := NewDotEnv(".env")
	state, err := LoadCodejobState(env)
	if err != nil {
		return "", err
	}

	// 1. If message provided -> close the loop
	if message != "" {
		if state.Phase != PhaseReview {
			return "", fmt.Errorf("no pending PR found in .env (run 'codejob' to check status)")
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

		// If -release flag is set and releaseFn is configured, create the release
		if isRelease && c.releaseFn != nil {
			if err := c.releaseFn(res.Tag); err != nil {
				return res.Summary, fmt.Errorf("release creation failed: %w", err)
			}
		}

		return res.Summary, nil
	}

	// 2. No message -> check status or auto-merge
	if state.Phase == PhaseRunning {
		return c.checkStatus(env, state)
	}

	// 3. Auto-merge pending PR before dispatching new work
	if state.Phase == PhaseReview {
		if c.publisher == nil {
			return "", fmt.Errorf("no publisher configured")
		}
		res, err := MergeAndPublish(c.publisher, "", "")
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

func (c *CodeJob) checkStatus(env *DotEnv, state CodejobState) (string, error) {
	driverName := state.Driver
	sessionID := state.Ref

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
		git, err := NewGit()
		if err != nil {
			return msg, fmt.Errorf("git init failed: %w", err)
		}
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
	if err := EnsureGHSession(); err != nil {
		return "", err
	}

	if _, err := ReadPlanMeta(issuePromptPath); err != nil {
		return "", fmt.Errorf("invalid plan frontmatter in %s: %w", issuePromptPath, err)
	}

	info, err := os.Stat(issuePromptPath)
	if err != nil {
		return "", fmt.Errorf("prompt file not found: %w", err)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("prompt file is empty: %s", issuePromptPath)
	}

	// PUBLISH BEFORE SEND (Stage 2, step 2.3)
	if c.publisher != nil {
		_, err := c.publisher.Publish("chore: sync before codejob dispatch", "", true, true, true, true, true, true)
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
					_ = SaveCodejobState(env, CodejobState{
						Driver: strings.ToLower(d.Name()),
						Phase:  PhaseRunning,
						Ref:    id,
					})
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
