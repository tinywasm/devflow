package devflow

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

const julesAPIKeyKey = "jules_api_key"
const julesAPIKeyURL = "https://jules.google.com/settings/api"

// termLink returns an OSC 8 terminal hyperlink (supported by most modern terminals).
func termLink(text, url string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// JulesAuth manages the Jules API key via the system keyring.
// On first use it prompts the user to enter the key and stores it securely.
type JulesAuth struct {
	kr  *Keyring
	log func(...any)
}

// NewJulesAuth creates a JulesAuth with an initialized keyring.
func NewJulesAuth() (*JulesAuth, error) {
	kr, err := NewKeyring()
	if err != nil {
		return nil, err
	}
	return &JulesAuth{
		kr:  kr,
		log: func(...any) {},
	}, nil
}

// SetLog sets the logging function.
func (a *JulesAuth) SetLog(fn func(...any)) {
	if fn != nil {
		a.log = fn
	}
}

// HasKey returns true if the Jules API key is already stored in the keyring.
func (a *JulesAuth) HasKey() bool {
	key, err := a.kr.Get(julesAPIKeyKey)
	return err == nil && key != ""
}

// EnsureAPIKey returns the Jules API key from the keyring.
// If absent, prompts the user for it once and persists it.
func (a *JulesAuth) EnsureAPIKey() (string, error) {
	key, err := a.kr.Get(julesAPIKeyKey)
	if err == nil && key != "" {
		return key, nil
	}

	fmt.Fprintf(os.Stderr, "Jules API Key not found. Get yours at %s\nEnter it now: ",
		termLink(julesAPIKeyURL, julesAPIKeyURL))
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("could not read API key: %w", err)
	}

	key = strings.TrimSpace(string(raw))
	if key == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	if err := a.kr.Set(julesAPIKeyKey, key); err != nil {
		a.log(fmt.Sprintf("warning: could not save API key to keyring: %v", err))
	}

	return key, nil
}
