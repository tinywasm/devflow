package devflow_test

import (
	"os"
	"testing"

	"github.com/tinywasm/devflow"
)

func TestSecretStore_Precedence(t *testing.T) {
	// Clean env
	os.Unsetenv("GH_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	defer os.Unsetenv("GH_TOKEN")
	defer os.Unsetenv("GITHUB_TOKEN")

	store := devflow.NewSecretStore()

	// 1. None
	_, _, err := store.Get(devflow.SecretGitHubToken)
	if err == nil {
		t.Error("expected error when no secret is present")
	}

	// 2. GITHUB_TOKEN
	os.Setenv("GITHUB_TOKEN", "token2")
	val, src, err := store.Get(devflow.SecretGitHubToken)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != "token2" {
		t.Errorf("expected token2, got %s", val)
	}
	if src != devflow.SourceEnv {
		t.Error("expected SourceEnv")
	}

	// 3. GH_TOKEN (precedence over GITHUB_TOKEN)
	os.Setenv("GH_TOKEN", "token1")
	val, src, err = store.Get(devflow.SecretGitHubToken)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != "token1" {
		t.Errorf("expected token1, got %s", val)
	}
}

func TestSecretStore_Trim(t *testing.T) {
	os.Setenv("JULES_API_KEY", "  key-with-spaces  \n")
	defer os.Unsetenv("JULES_API_KEY")

	store := devflow.NewSecretStore()
	val, _, err := store.Get(devflow.SecretJulesAPIKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "key-with-spaces" {
		t.Errorf("expected trimmed key, got %q", val)
	}

	os.Setenv("JULES_API_KEY", "   ")
	_, _, err = store.Get(devflow.SecretJulesAPIKey)
	if err == nil {
		t.Error("expected error for empty-space env var")
	}
}

func TestSecretStore_UnknownSecret(t *testing.T) {
	store := devflow.NewSecretStore()
	_, _, err := store.Get("non-existent")
	if err == nil {
		t.Error("expected error for unknown secret")
	}
}
