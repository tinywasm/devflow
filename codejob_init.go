package devflow

import (
	"fmt"
	"os"
	"strings"

	"github.com/tinywasm/context"
	"github.com/tinywasm/wizard"
	"golang.org/x/term"
)

// CodeJobInitWizard implements the one-time setup wizard for codejob.
// It follows the same wizard.Step pattern as gonew_wizard.go.
type CodeJobInitWizard struct {
	log     func(...any)
	stepErr error // captures fatal errors from steps for Run() to inspect
}

// NewCodeJobInitWizard creates a CodeJobInitWizard.
func NewCodeJobInitWizard() *CodeJobInitWizard {
	return &CodeJobInitWizard{log: func(...any) {}}
}

// SetLog sets the logging function.
func (c *CodeJobInitWizard) SetLog(fn func(...any)) {
	if fn != nil {
		c.log = fn
	}
}

// Run drives the init wizard from a CLI context.
// It handles masked input for the API key step.
func (c *CodeJobInitWizard) Run() error {
	wiz := wizard.New(func(_ *context.Context) {
		fmt.Println("\n✅ Jules API key saved. Run 'codejob' to dispatch a task.")
	}, c)

	// Step 1: Jules API key — masked read, retries on validation failure.
	// This is Jules domain knowledge: the key requires masked input.
	for wiz.WaitingForUser() {
		label := wiz.Label()
		fmt.Fprintf(os.Stderr, "\n%s: ", label)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("could not read API key: %w", err)
		}
		wiz.Change(string(raw))
		if c.stepErr != nil {
			return c.stepErr
		}
		if wiz.Label() != label || !wiz.WaitingForUser() {
			break // step advanced
		}
	}

	return nil
}

// GetSteps returns the single onboarding step: save the Jules API key to keyring.
func (c *CodeJobInitWizard) GetSteps() []*wizard.Step {
	return []*wizard.Step{
		// Step 1: Jules API Key — input is delivered masked by Run().
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
					c.stepErr = fmt.Errorf("could not initialize keyring: %w", err)
					return false, c.stepErr
				}
				if err := kr.Set(julesAPIKeyKey, in); err != nil {
					c.log(fmt.Sprintf("warning: could not save API key to keyring: %v", err))
				}
				return true, nil
			},
		},
	}
}
