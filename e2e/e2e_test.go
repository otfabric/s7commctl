// SPDX-License-Identifier: MIT

//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// run executes the binary named by S7COMMCTL_E2E_BIN (set by make e2e).
func run(t *testing.T, args ...string) (output string, exitCode int) {
	t.Helper()
	return runEnv(t, nil, args...)
}

// runEnv is run with env ("NAME=value") added to the environment.
func runEnv(t *testing.T, env []string, args ...string) (output string, exitCode int) {
	t.Helper()
	bin := os.Getenv("S7COMMCTL_E2E_BIN")
	if bin == "" {
		t.Fatal("S7COMMCTL_E2E_BIN is not set; run make e2e")
	}
	var out bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &exit):
		return out.String(), exit.ExitCode()
	default:
		t.Fatalf("run %s: %v", bin, err)
		return "", -1
	}
}

func TestVersionPrintsTheBuiltVersion(t *testing.T) {
	output, code := run(t, "version")
	if code != 0 {
		t.Fatalf("exit code %d, want 0\n%s", code, output)
	}
	// make e2e sets the variable to the tag of HEAD, empty when HEAD has none: then dev.
	want, ok := os.LookupEnv("S7COMMCTL_E2E_VERSION")
	if !ok {
		t.Fatal("S7COMMCTL_E2E_VERSION is not set; run make e2e")
	}
	if want = strings.TrimPrefix(want, "v"); want == "" {
		want = "dev"
	}
	if !strings.HasPrefix(output, "s7commctl "+want+" (") || strings.Count(output, "\n") != 1 ||
		!strings.Contains(output, ", go-s7comm ") {
		t.Fatalf("output is not the line of s7commctl %s:\n%s", want, output)
	}
	output, code = run(t, "version", "--json")
	if code != 0 || !strings.HasPrefix(output, `{"name":"s7commctl","version":"`+want+`","tag":`) {
		t.Fatalf("version --json: exit %d\n%s", code, output)
	}
}

// An unknown command is a usage error: exit code 2 in docs/exitcodes.md.
func TestUnknownCommandIsAUsageError(t *testing.T) {
	output, code := run(t, "nosuchcommand")
	if code != 2 {
		t.Fatalf("exit code %d, want 2\n%s", code, output)
	}
	if !strings.Contains(output, `unknown command "nosuchcommand"`) {
		t.Fatalf("output does not name the unknown command:\n%s", output)
	}
}

// S7COMMCTL_REQUEST_TIMEOUT sets --request-timeout: a negative value is a
// usage error (exit 2) before connecting, as on the command line.
func TestRequestTimeoutFromTheEnvironment(t *testing.T) {
	output, code := runEnv(t, []string{"S7COMMCTL_REQUEST_TIMEOUT=-1"}, "identify", "--ip", "127.0.0.1", "--port", "1")
	if code != 2 {
		t.Fatalf("exit code %d, want 2\n%s", code, output)
	}
	if !strings.Contains(output, "request-timeout must be >= 0 (0: no bound per request), got -1") {
		t.Fatalf("output does not name --request-timeout:\n%s", output)
	}
}
