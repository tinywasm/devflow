package devflow

import (
	"fmt"
	"os"
	"strings"
)

// DefaultIssuePromptPath is the conventional location for the task description file.
const DefaultIssuePromptPath = "docs/PLAN.md"

// CodeJob orchestrates sending a coding task to a chain of AI agent drivers.
// It validates the prompt file, then tries each driver in priority order,
// falling back to the next on failure.
type CodeJob struct {
	drivers []CodeJobDriver
	sync    RepoSync
	log     func(...any)
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

// SetRepoSync injects a RepoSync for pre-flight synchronization check.
// When set, Send() will refuse to dispatch if the local repo is not in sync with the remote.
func (c *CodeJob) SetRepoSync(s RepoSync) { c.sync = s }

// Send validates issuePromptPath, checks repo sync, then tries each
// driver in order until one succeeds. Returns an error if the file is missing,
// empty, the repo is out of sync, or all drivers fail.
func (c *CodeJob) Send(issuePromptPath string) (string, error) {
	info, err := os.Stat(issuePromptPath)
	if err != nil {
		return "", fmt.Errorf("prompt file not found: %w", err)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("prompt file is empty: %s", issuePromptPath)
	}

	if c.sync != nil {
		pending, err := c.sync.HasPendingChanges()
		if err != nil {
			return "", fmt.Errorf("repo sync check failed: %w", err)
		}
		if pending {
			return "", fmt.Errorf(
				"repository is not in sync with remote — Jules reads from GitHub, not the local filesystem",
			)
		}
	}

	if len(c.drivers) == 0 {
		return "", fmt.Errorf("no drivers configured")
	}

	prompt := "Execute the implementation plan described in " + issuePromptPath

	var lastErr error
	for _, d := range c.drivers {
		d.SetLog(c.log)
		result, err := d.Send(prompt)
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

// TryDispatch dispatches CodeJob if PLAN.md exists and no active session is in .env.
// It returns a non-empty string on successful dispatch, or "" if nothing was done.
func TryDispatch(sync RepoSync) string {
	env := NewDotEnv(".env")
	if _, ok := env.Get("CODEJOB"); ok {
		return "" // already active
	}

	if _, err := os.Stat(DefaultIssuePromptPath); os.IsNotExist(err) {
		return "" // no plan to dispatch
	}

	job := NewCodeJob(NewJulesDriver(JulesConfig{}))
	job.SetRepoSync(sync)

	result, err := job.Send(DefaultIssuePromptPath)
	if err != nil {
		return "" // silent failure for try dispatch
	}

	// Read back from env because result might be verbose, but we want the persisted session id
	if val, ok := env.Get("CODEJOB"); ok {
		parts := strings.SplitN(val, ":", 2)
		if len(parts) == 2 {
			return "→ Jules: " + parts[1]
		}
	}

	return result
}
