package devflow_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tinywasm/devflow"
)

// mockHTTPClient is a test double for devflow.HTTPClient.
type mockHTTPClient struct {
	statusCode int
	body       string
	err        error
	lastReq    *http.Request
	lastBody   []byte
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	if req.Body != nil {
		m.lastBody, _ = io.ReadAll(req.Body)
	}
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

func testJulesConfig() devflow.JulesConfig {
	return devflow.JulesConfig{
		APIKey:      "test-key",
		SourceID:    "sources/github/user/repo",
		StartBranch: "main",
	}
}

func TestJulesDriverSendSuccess(t *testing.T) {
	d := devflow.NewJulesDriver(testJulesConfig())
	d.SetHTTPClient(&mockHTTPClient{statusCode: 200, body: `{"id":"S123"}`})

	result, err := d.Send("Execute the implementation plan described in docs/PLAN.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "→ Jules: S123") {
		t.Errorf("unexpected result: %s", result)
	}
	if d.SessionID() != "S123" {
		t.Errorf("expected session ID S123, got %q", d.SessionID())
	}
}

func TestJulesDriverSendNon200(t *testing.T) {
	d := devflow.NewJulesDriver(testJulesConfig())
	d.SetHTTPClient(&mockHTTPClient{statusCode: 403, body: "forbidden"})

	_, err := d.Send("Execute the implementation plan described in docs/PLAN.md")
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestJulesDriverSendUsesProvidedAPIKey(t *testing.T) {
	mock := &mockHTTPClient{statusCode: 200, body: "{}"}
	d := devflow.NewJulesDriver(testJulesConfig())
	d.SetHTTPClient(mock)

	d.Send("Execute the implementation plan described in docs/PLAN.md")

	if mock.lastReq == nil {
		t.Fatal("no request was made")
	}
	if got := mock.lastReq.Header.Get("X-Goog-Api-Key"); got != "test-key" {
		t.Errorf("expected API key %q, got %q", "test-key", got)
	}
}

func TestJulesDriverSendUsesReceivedPrompt(t *testing.T) {
	const customPrompt = "Execute the implementation plan described in docs/PLAN.md"
	mock := &mockHTTPClient{statusCode: 200, body: "{}"}
	d := devflow.NewJulesDriver(testJulesConfig())
	d.SetHTTPClient(mock)

	d.Send(customPrompt)

	if mock.lastBody == nil {
		t.Fatal("no request body captured")
	}

	var payload map[string]any
	if err := json.Unmarshal(mock.lastBody, &payload); err != nil {
		t.Fatalf("could not decode request body: %v", err)
	}

	got, ok := payload["prompt"].(string)
	if !ok || got == "" {
		t.Fatal("prompt field missing or empty in request body")
	}
	if got != customPrompt {
		t.Errorf("driver must use the prompt it received\nwant: %q\n got: %q", customPrompt, got)
	}
}
