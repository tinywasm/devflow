package devflow_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

// fakeRunner captures the args of the last executed command
// and returns configured output/error.
type fakeRunner struct {
	lastArgs  []string
	lastInput string
	output    string
	err       error
}

func (f *fakeRunner) Run(name string, args ...string) (string, error) {
	f.lastArgs = args
	return f.output, f.err
}

func (f *fakeRunner) RunSilent(name string, args ...string) (string, error) {
	f.lastArgs = args
	return f.output, f.err
}

func (f *fakeRunner) RunWithStdin(input, name string, args ...string) (string, error) {
	f.lastInput = input
	f.lastArgs = args
	return f.output, f.err
}

// newTestGitHub creates a *GitHub with injected fakeRunner.
func newTestGitHub(fake *fakeRunner) *devflow.GitHub {
	gh := &devflow.GitHub{}
	gh.SecretRunner = fake
	return gh
}

func TestSetSecret_SecurityAndArgs(t *testing.T) {
	fake := &fakeRunner{}
	gh := newTestGitHub(fake)

	secretValue := "abc123"
	err := gh.SetSecret("owner/repo", "CF_TOKEN", secretValue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedArgs := []string{"secret", "set", "CF_TOKEN", "--body-file", "-", "--repo", "owner/repo"}
	if !reflect.DeepEqual(fake.lastArgs, expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, fake.lastArgs)
	}

	if fake.lastInput != secretValue {
		t.Errorf("expected input %q, got %q", secretValue, fake.lastInput)
	}
}

func TestSetSecret_ErrorWrapped(t *testing.T) {
	fake := &fakeRunner{err: fmt.Errorf("gh failed")}
	gh := newTestGitHub(fake)

	err := gh.SetSecret("owner/repo", "CF_TOKEN", "abc123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "CF_TOKEN") || !strings.Contains(err.Error(), "owner/repo") {
		t.Errorf("error message should contain secret name and repo: %v", err)
	}
}

func TestListSecrets_ParsesFlatArray(t *testing.T) {
	fake := &fakeRunner{output: `["CF_TOKEN","GH_PAT"]`}
	gh := newTestGitHub(fake)

	names, err := gh.ListSecrets("owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedNames := []string{"CF_TOKEN", "GH_PAT"}
	if !reflect.DeepEqual(names, expectedNames) {
		t.Errorf("expected names %v, got %v", expectedNames, names)
	}
}

func TestListSecrets_EmptyArray(t *testing.T) {
	fake := &fakeRunner{output: `[]`}
	gh := newTestGitHub(fake)

	names, err := gh.ListSecrets("owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(names) != 0 {
		t.Errorf("expected empty names, got %v", names)
	}
}

func TestListSecrets_ParseError(t *testing.T) {
	fake := &fakeRunner{output: `invalid json`}
	gh := newTestGitHub(fake)

	_, err := gh.ListSecrets("owner/repo")
	if err == nil {
		t.Fatal("expected error due to invalid JSON, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse secrets list JSON") {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

func TestDeleteSecret_CommandArgs(t *testing.T) {
	fake := &fakeRunner{}
	gh := newTestGitHub(fake)

	err := gh.DeleteSecret("owner/repo", "CF_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedArgs := []string{"secret", "delete", "CF_TOKEN", "--repo", "owner/repo"}
	if !reflect.DeepEqual(fake.lastArgs, expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, fake.lastArgs)
	}
}

func TestDeleteSecret_ErrorWrapped(t *testing.T) {
	fake := &fakeRunner{err: fmt.Errorf("gh failed")}
	gh := newTestGitHub(fake)

	err := gh.DeleteSecret("owner/repo", "CF_TOKEN")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "CF_TOKEN") || !strings.Contains(err.Error(), "owner/repo") {
		t.Errorf("error message should contain secret name and repo: %v", err)
	}
}
