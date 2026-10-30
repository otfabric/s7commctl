// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestMain_Version runs main in-process: main hands the link-time variables to
// the command tree.
func TestMain_Version(t *testing.T) {
	oldArgs, oldVersion, oldStdout := os.Args, version, os.Stdout
	t.Cleanup(func() { os.Args, version, os.Stdout = oldArgs, oldVersion, oldStdout })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"s7commctl", "version"}
	version = "9.9.9-test"
	os.Stdout = w
	main()
	os.Stdout = oldStdout
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "s7commctl version 9.9.9-test") {
		t.Fatalf("stdout = %q", out)
	}
}

// TestBinary_ExitCodes builds the binary and runs it: only a real process shows
// that Execute turns errors into exit codes and keeps usage off runtime failures.
func TestBinary_ExitCodes(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "s7commctl")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()

	run := func(args ...string) (code int, stdout, stderr string) {
		t.Helper()
		var outBuf, errBuf bytes.Buffer
		cmd := exec.Command(bin, args...)
		cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
		err := cmd.Run()
		var ee *exec.ExitError
		switch {
		case err == nil:
		case errors.As(err, &ee):
			code = ee.ExitCode()
		default:
			t.Fatalf("run %v: %v", args, err)
		}
		return code, outBuf.String(), errBuf.String()
	}

	if code, stdout, _ := run("version"); code != 0 || !strings.HasPrefix(stdout, "s7commctl version ") {
		t.Errorf("version: exit %d, stdout %q", code, stdout)
	}

	if code, _, stderr := run("read"); code != 2 || !strings.Contains(stderr, "IP address is required") {
		t.Errorf("read without --ip: exit %d, stderr %q", code, stderr)
	}

	code, _, stderr := run("read", "--ip", "127.0.0.1", "--port", closedPort, "--timeout", "500")
	if code < 3 || !strings.Contains(stderr, "connect failed") {
		t.Errorf("connection refused: exit %d, stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "Usage:") {
		t.Errorf("usage printed on a runtime failure:\n%s", stderr)
	}

	if code, _, stderr := run("--no-such-flag"); code == 0 || !strings.Contains(stderr, "unknown flag") {
		t.Errorf("unknown flag: exit %d, stderr %q", code, stderr)
	}
}
