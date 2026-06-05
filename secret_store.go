package devflow

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// SecretSource indicates where a secret was resolved from.
type SecretSource int

const (
	SourceNone    SecretSource = iota // not found
	SourceEnv                         // environment variable
	SourceKeyring                     // system keyring
)

// secretSpec describes how to resolve a logical secret.
type secretSpec struct {
	keyringKey string   // key in the keyring (compatibility with existing stored values)
	envKeys    []string // environment variables to try, in priority order
}

// Logical names of managed secrets.
const (
	SecretGitHubToken = "github_token"
	SecretJulesAPIKey = "jules_api_key"
)

// secretRegistry maps each logical secret to its spec. It is the SINGLE source of truth
// for env var names and keyring keys. Do not duplicate these literals in other files.
var secretRegistry = map[string]secretSpec{
	SecretGitHubToken: {keyringKey: "github_token", envKeys: []string{"GH_TOKEN", "GITHUB_TOKEN"}},
	SecretJulesAPIKey: {keyringKey: "jules_api_key", envKeys: []string{"JULES_API_KEY"}},
}

// ErrSecretNotFound is returned when a known secret is not found in any backend.
var ErrSecretNotFound = errors.New("secret not found in environment or keyring")

// SecretStore resolves credentials with environment → keyring precedence.
// The keyring is initialized lazily: it is NEVER touched if the credential
// is available via environment variable (key for CI/CD).
type SecretStore struct {
	log  func(...any)
	kr   *Keyring
	mu   sync.Mutex
	once sync.Once
}

// NewSecretStore creates the handler. It does NOT initialize the keyring (no cost or side effects).
func NewSecretStore() *SecretStore {
	return &SecretStore{
		log: func(...any) {},
	}
}

// SetLog assigns the logger (propagates to Keyring when initialized).
func (s *SecretStore) SetLog(fn func(...any)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = fn
	if s.kr != nil {
		s.kr.SetLog(fn)
	}
}

func (s *SecretStore) keyring() (*Keyring, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kr == nil {
		kr, err := NewKeyring()
		if err != nil {
			return nil, err
		}
		s.kr = kr
		s.kr.SetLog(s.log)
	}
	return s.kr, nil
}

// Get resolves the value of secret `name`. Returns value, its source and error.
//   - If name is not in secretRegistry → error (programming).
//   - If found in env → (value, SourceEnv, nil) WITHOUT touching keyring.
//   - If not in env but in keyring → (value, SourceKeyring, nil).
//   - If not in any backend → ("", SourceNone, ErrSecretNotFound).
func (s *SecretStore) Get(name string) (string, SecretSource, error) {
	spec, ok := secretRegistry[name]
	if !ok {
		return "", SourceNone, fmt.Errorf("unknown secret %q", name)
	}

	for _, e := range spec.envKeys {
		if v := strings.TrimSpace(os.Getenv(e)); v != "" {
			return v, SourceEnv, nil
		}
	}

	kr, err := s.keyring()
	if err != nil {
		return "", SourceNone, err
	}

	v, err := kr.Get(spec.keyringKey)
	if err == nil && v != "" {
		return v, SourceKeyring, nil
	}

	return "", SourceNone, ErrSecretNotFound
}

// Set persists the value in the keyring (only writable backend).
// Used by interactive acquisition after obtaining a new secret.
func (s *SecretStore) Set(name, value string) error {
	spec, ok := secretRegistry[name]
	if !ok {
		return fmt.Errorf("unknown secret %q", name)
	}

	kr, err := s.keyring()
	if err != nil {
		return err
	}

	return kr.Set(spec.keyringKey, value)
}

// Delete removes the value from the keyring (e.g. invalid token).
func (s *SecretStore) Delete(name string) error {
	spec, ok := secretRegistry[name]
	if !ok {
		return fmt.Errorf("unknown secret %q", name)
	}

	kr, err := s.keyring()
	if err != nil {
		return err
	}

	return kr.Delete(spec.keyringKey)
}

// IsInteractive indicates if there is a TTY where credentials can be requested from the user.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
