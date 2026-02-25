package devflow_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

// mockCodeJobDriver is a test double for devflow.CodeJobDriver.
type mockCodeJobDriver struct {
	name   string
	result string
	err    error
}

func (m *mockCodeJobDriver) Name() string                  { return m.name }
func (m *mockCodeJobDriver) SetLog(_ func(...any))         {}
func (m *mockCodeJobDriver) Send(_ string) (string, error) { return m.result, m.err }

// writeTempFile creates a temp file with content and registers cleanup.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "codejob-*.md")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestCodeJobSendFirstDriverSuccess(t *testing.T) {
	path := writeTempFile(t, "some plan")
	d := &mockCodeJobDriver{name: "mock", result: "ok"}
	job := devflow.NewCodeJob(d)

	// mock doesn't implement SessionProvider, so no persistence expected
	got, err := job.Send(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
}

type mockSessionDriver struct {
	mockCodeJobDriver
	sessionID string
}

func (m *mockSessionDriver) SessionID() string { return m.sessionID }

func TestCodeJobSendPersistsSession(t *testing.T) {
	path := writeTempFile(t, "some plan")
	envPath := ".env"
	defer os.Remove(envPath)

	d := &mockSessionDriver{
		mockCodeJobDriver: mockCodeJobDriver{name: "jules", result: "ok"},
		sessionID:         "S123",
	}
	job := devflow.NewCodeJob(d)

	_, err := job.Send(path)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "CODEJOB=jules:S123") {
		t.Errorf("expected session persisted to .env, got: %s", string(data))
	}
}

func TestCodeJobSendFallsBackToSecondDriver(t *testing.T) {
	path := writeTempFile(t, "some plan")
	d1 := &mockCodeJobDriver{name: "fail", err: errors.New("down")}
	d2 := &mockCodeJobDriver{name: "ok", result: "fallback"}
	job := devflow.NewCodeJob(d1, d2)

	got, err := job.Send(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", got)
	}
}

func TestCodeJobSendAllDriversFail(t *testing.T) {
	path := writeTempFile(t, "some plan")
	errB := errors.New("err b")
	d1 := &mockCodeJobDriver{name: "a", err: errors.New("err a")}
	d2 := &mockCodeJobDriver{name: "b", err: errB}
	job := devflow.NewCodeJob(d1, d2)

	_, err := job.Send(path)
	if err == nil {
		t.Fatal("expected error when all drivers fail")
	}
	if !errors.Is(err, errB) {
		t.Errorf("expected last error to be wrapped, got: %v", err)
	}
}

func TestCodeJobSendNoDrivers(t *testing.T) {
	path := writeTempFile(t, "some plan")
	job := devflow.NewCodeJob()

	_, err := job.Send(path)
	if err == nil {
		t.Fatal("expected error when no drivers configured")
	}
}

func TestCodeJobSendFileNotFound(t *testing.T) {
	job := devflow.NewCodeJob(&mockCodeJobDriver{name: "mock", result: "ok"})

	_, err := job.Send("/nonexistent/PLAN.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestCodeJobSendFileEmpty(t *testing.T) {
	path := writeTempFile(t, "") // empty file
	job := devflow.NewCodeJob(&mockCodeJobDriver{name: "mock", result: "ok"})

	_, err := job.Send(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}

// mockRepoSync is a test double for devflow.RepoSync.
type mockRepoSync struct {
	hasPending bool
	err        error
}

func (m *mockRepoSync) HasPendingChanges() (bool, error) { return m.hasPending, m.err }

func TestCodeJobSendBlockedWhenPendingChanges(t *testing.T) {
	path := writeTempFile(t, "some plan")
	job := devflow.NewCodeJob(&mockCodeJobDriver{name: "mock", result: "ok"})
	job.SetRepoSync(&mockRepoSync{hasPending: true})

	_, err := job.Send(path)
	if err == nil {
		t.Fatal("expected error when pending changes exist")
	}
	if !strings.Contains(err.Error(), "not in sync") {
		t.Errorf("expected sync error message, got: %v", err)
	}
}

func TestCodeJobSendAllowedWhenNoPendingChanges(t *testing.T) {
	path := writeTempFile(t, "some plan")
	job := devflow.NewCodeJob(&mockCodeJobDriver{name: "mock", result: "dispatched"})
	job.SetRepoSync(&mockRepoSync{hasPending: false})

	got, err := job.Send(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "dispatched" {
		t.Errorf("expected %q, got %q", "dispatched", got)
	}
}

func TestCodeJobSendSkipsGitCheckWithNoClient(t *testing.T) {
	path := writeTempFile(t, "some plan")
	// No SetRepoSync call — sync check must be silently skipped.
	job := devflow.NewCodeJob(&mockCodeJobDriver{name: "mock", result: "ok"})

	got, err := job.Send(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
}
