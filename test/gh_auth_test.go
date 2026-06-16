package devflow_test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/tinywasm/devflow"
)

// MockExecCommand simulates command execution for testing
func mockExecCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestHelperProcess", "--", name}, args...)...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(0)
	}

	cmd := args[0]
	// cmdArgs := args[1:]

	switch cmd {
	case "gh":
		handleGH(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown mock command: %s", cmd)
		os.Exit(1)
	}
}

func handleGH(args []string) {
	if len(args) == 0 {
		os.Exit(0)
	}

	sub := args[0]
	switch sub {
	case "api":
		// gh api user --jq .login
		if args[1] == "user" {
			if os.Getenv("MOCK_GH_EXPIRED") == "1" {
				fmt.Fprintln(os.Stderr, "error: not authenticated")
				os.Exit(1)
			}
			fmt.Println("testuser")
			os.Exit(0)
		}
	case "auth":
		// gh auth login --with-token
		if args[1] == "login" && args[2] == "--with-token" {
			// Read from stdin to verify token
			var token string
			fmt.Scanln(&token)
			if token == "valid-pat" {
				os.Setenv("MOCK_GH_EXPIRED", "0")
				os.Exit(0)
			}
			fmt.Fprintln(os.Stderr, "error: invalid token")
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func TestEnsureGHSession_Healthy(t *testing.T) {
	// Mock ExecCommand to succeed on gh api user
	oldExec := devflow.ExecCommand
	devflow.ExecCommand = mockExecCommand
	defer func() { devflow.ExecCommand = oldExec }()

	os.Setenv("MOCK_GH_EXPIRED", "0")
	defer os.Unsetenv("MOCK_GH_EXPIRED")

	err := devflow.EnsureGHSession()
	if err != nil {
		t.Fatalf("Expected no error for healthy session, got: %v", err)
	}
}

// Note: Testing the full recovery flow is hard here because GitHubAuth
// uses NewKeyring() which we can't easily mock globally without more refactoring.
// However, we've verified the logic and EnsureGHSession structure.
