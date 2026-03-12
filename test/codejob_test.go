package devflow_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

type mockDriver struct {
	name   string
	result string
	err    error
}

func (m *mockDriver) Name() string                  { return m.name }
func (m *mockDriver) SetLog(_ func(...any))         {}
func (m *mockDriver) Send(_, _ string) (string, error) { return m.result, m.err }

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

func TestCodeJob_Run_NoArgs_Dispatch(t *testing.T) {
	dir := t.TempDir()
	defer testChdir(t, dir)()
	os.MkdirAll("docs", 0755)
	os.WriteFile("docs/PLAN.md", []byte("some plan"), 0644)

	d := &mockDriver{name: "mock", result: "ok"}
	job := devflow.NewCodeJob(d)

    // Mock Publisher to satisfy Send's publish-before-dispatch
    job.SetPublisher(&MockPublisher{})

	got, err := job.Run("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected ok, got %q", got)
	}
}

func TestCodeJob_Run_WithMessage_CloseLoop(t *testing.T) {
	// Logic moved to codejob_state_test.go because it requires complex mocking of 'gh' and 'git' commands
}

func TestCodeJob_MessageWithoutPR(t *testing.T) {
	job := devflow.NewCodeJob()
	_, err := job.Run("some message", "")
	if err == nil {
		t.Fatal("expected error when no PR found")
	}
	if !strings.Contains(err.Error(), "no pending PR found") {
		t.Errorf("expected error message about missing PR, got: %v", err)
	}
}

func TestCodeJob_Send_PublishesBeforeDispatch(t *testing.T) {
	path := writeTempFile(t, "some plan")
	published := false
	mockPub := &MockPublisher{
		PublishFn: func(m, tag string, st, sr, sd, sb, stag bool) (devflow.PushResult, error) {
			published = true
			if !stag || !sd || !sb {
				t.Errorf("expected skipTag, skipDependents, skipBackup to be true")
			}
			return devflow.PushResult{}, nil
		},
	}
	d := &mockDriver{name: "mock", result: "ok"}
	job := devflow.NewCodeJob(d)
	job.SetPublisher(mockPub)

	_, err := job.Send(path)
	if err != nil {
		t.Fatal(err)
	}
	if !published {
		t.Error("Publish should have been called before Send")
	}
}

func TestCodeJob_Send_PublishSilently(t *testing.T) {
	// Verify that Publish is called silently (no logging of summary)
	path := writeTempFile(t, "some plan")
	publishCalled := false
	mockPub := &MockPublisher{
		PublishFn: func(m, tag string, st, sr, sd, sb, stag bool) (devflow.PushResult, error) {
			publishCalled = true
			return devflow.PushResult{Summary: "Pushed ✅ v1.2.3"}, nil
		},
	}
	var logged []string
	d := &mockDriver{name: "mock", result: "ok"}
	job := devflow.NewCodeJob(d)
	job.SetPublisher(mockPub)
	job.SetLog(func(args ...any) { logged = append(logged, fmt.Sprint(args...)) })

	_, err := job.Send(path)
	if err != nil {
		t.Fatal(err)
	}
	if !publishCalled {
		t.Error("Publish should have been called")
	}
	if len(logged) > 0 {
		t.Errorf("expected no logging of publish summary, but got: %v", logged)
	}
}
