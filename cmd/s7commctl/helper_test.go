// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// cliResult is the outcome of one in-process invocation.
type cliResult struct {
	stdout, stderr string
	err            error
}

// runCLI runs the command tree with args, as the process would, and captures
// stdout and stderr. Commands keep their flags in package variables, so tests
// using it cannot run in parallel.
func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	return runCLIContext(t, context.Background(), args...)
}

func runCLIContext(t *testing.T, ctx context.Context, args ...string) cliResult {
	t.Helper()
	resetCommands(ctx)
	t.Cleanup(func() { resetCommands(context.Background()) })
	var r cliResult
	r.stdout, r.stderr = captureOutput(t, func() {
		rootCmd.SetArgs(args)
		r.err = rootCmd.ExecuteContext(ctx)
	})
	return r
}

// resetCommands returns every flag to its default and gives every command ctx:
// cobra keeps both on the command between executions.
func resetCommands(ctx context.Context) {
	rootCmd.SetArgs(nil)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.SetContext(ctx)
		for _, f := range collect(c.Flags().VisitAll) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}

// collect gathers what visit yields. It keeps pflag, an indirect dependency, out
// of this file's imports.
func collect[T any](visit func(func(T))) []T {
	var out []T
	visit(func(v T) { out = append(out, v) })
	return out
}

// captureOutput runs fn with os.Stdout and os.Stderr redirected.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	var outBuf, errBuf bytes.Buffer
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(&outBuf, outR); done <- struct{}{} }()
	go func() { _, _ = io.Copy(&errBuf, errR); done <- struct{}{} }()

	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	fn()
	_ = outW.Close()
	_ = errW.Close()
	<-done
	<-done
	return outBuf.String(), errBuf.String()
}

// withStdin makes input the content of os.Stdin for the rest of the test.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := io.WriteString(w, input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		_ = r.Close()
	})
}

// closedPort returns a loopback TCP port nothing listens on.
func closedPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func (r cliResult) ok(t *testing.T) cliResult {
	t.Helper()
	if r.err != nil {
		t.Fatalf("unexpected error: %v\nstdout:\n%s\nstderr:\n%s", r.err, r.stdout, r.stderr)
	}
	return r
}

// has fails the test unless stdout contains every one of subs.
func (r cliResult) has(t *testing.T, subs ...string) cliResult {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("stdout does not contain %q:\n%s", s, r.stdout)
		}
	}
	return r
}

// lacks fails the test if stdout contains one of subs.
func (r cliResult) lacks(t *testing.T, subs ...string) cliResult {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(r.stdout, s) {
			t.Errorf("stdout contains %q:\n%s", s, r.stdout)
		}
	}
	return r
}

// exitCodeOf returns the code of an *ExitError and fails the test on anything else.
func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	return ee.Code
}

// wantUsage asserts exit code 2 and an error message containing msg.
func (r cliResult) wantUsage(t *testing.T, msg string) cliResult {
	t.Helper()
	return r.wantCode(t, msg, func(code int) bool { return code == 2 }, "2 (usage)")
}

// wantFindings asserts exit code 1: the operation ran and reports a non-success.
func (r cliResult) wantFindings(t *testing.T, msg string) cliResult {
	t.Helper()
	return r.wantCode(t, msg, func(code int) bool { return code == 1 }, "1 (non-success)")
}

// wantFailure asserts a connection or runtime failure. These all exit 3 today;
// docs/exitcodes.md is moving them to 4, 5 and 6, so the class is asserted here
// and the exact mapping belongs to the exit-code tests.
func (r cliResult) wantFailure(t *testing.T, msg string) cliResult {
	t.Helper()
	return r.wantCode(t, msg, func(code int) bool { return code >= 3 }, ">= 3 (connection or runtime)")
}

func (r cliResult) wantCode(t *testing.T, msg string, match func(int) bool, want string) cliResult {
	t.Helper()
	if r.err == nil {
		t.Fatalf("expected an error containing %q\nstdout:\n%s", msg, r.stdout)
	}
	if code := exitCodeOf(t, r.err); !match(code) {
		t.Errorf("exit code %d, want %s (error: %v)", code, want, r.err)
	}
	if !strings.Contains(r.err.Error(), msg) {
		t.Errorf("error %q does not contain %q", r.err, msg)
	}
	return r
}

// jsonOut decodes stdout as one JSON object.
func (r cliResult) jsonOut(t *testing.T) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(r.stdout), &m); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, r.stdout)
	}
	return m
}

// field walks nested JSON objects along path and fails the test if one is missing.
func field(t *testing.T, m map[string]interface{}, path ...string) interface{} {
	t.Helper()
	var cur interface{} = m
	for _, p := range path {
		obj, ok := cur.(map[string]interface{})
		if !ok {
			t.Fatalf("%v: %q is not inside an object: %v", path, p, cur)
		}
		if cur, ok = obj[p]; !ok {
			t.Fatalf("%v: no key %q in %v", path, p, obj)
		}
	}
	return cur
}

func wantField(t *testing.T, m map[string]interface{}, want interface{}, path ...string) {
	t.Helper()
	if got := field(t, m, path...); got != want {
		t.Errorf("%v = %v (%T), want %v (%T)", path, got, got, want, want)
	}
}
