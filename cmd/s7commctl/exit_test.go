// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"testing"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/wire"
)

func TestExitError_UsageCode(t *testing.T) {
	err := ExitUsage(errors.New("bad input"))
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatal("expected *ExitError")
	}
	if ee.Code != 2 {
		t.Errorf("expected code 2 (usage), got %d", ee.Code)
	}
}

func TestExitError_OperationCode(t *testing.T) {
	err := ExitOperation(errors.New("no valid rack/slot"))
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatal("expected *ExitError")
	}
	if ee.Code != 1 {
		t.Errorf("expected code 1 (operation), got %d", ee.Code)
	}
}

func TestExitError_RuntimeCode(t *testing.T) {
	err := ExitRuntime(errors.New("connect failed"))
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatal("expected *ExitError")
	}
	if ee.Code != 6 {
		t.Errorf("expected code 6 (protocol), got %d", ee.Code)
	}
}

func TestReadCommand_NoIP_ReturnsUsageExit(t *testing.T) {
	rootCmd.SetArgs([]string{"read"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when running read without IP")
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("expected exit code 2 (usage), got %d", ee.Code)
	}
}

// TestCompareRead_InvalidCandidates_ReturnsUsageExit exercises parseCandidates error path.
func TestCompareRead_InvalidCandidates_ReturnsUsageExit(t *testing.T) {
	rootCmd.SetArgs([]string{"compare", "read", "--ip", "127.0.0.1", "--area", "I", "--offset", "0", "--size", "4", "--candidates", "x/y"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid candidates")
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *ExitError: %T %v", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("expected exit code 2 (usage), got %d", ee.Code)
	}
}

func TestClassifiedExitCodes(t *testing.T) {
	refused := &net.OpError{Op: "dial", Err: errors.New("refused")}
	for _, c := range []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"cobra", errors.New("unknown flag: --x"), 2},
		{"operation", ExitOperation(errors.New("short read")), 1},
		{"connect", ExitConnect(errors.New("connect failed")), 4},
		{"connect refused", ExitConnect(fmt.Errorf("connect failed: %w", refused)), 4},
		{"connect timeout", ExitConnect(fmt.Errorf("connect failed: %w", context.DeadlineExceeded)), 5},
		{"interrupted", ExitRuntime(fmt.Errorf("read failed: %w", context.Canceled)), 130},
		{"not connected", ExitRuntime(client.ErrNotConnected), 4},
		{"protocol", ExitRuntime(client.ErrProtocolFailure), 6},
		{"truncated", ExitRuntime(wire.ErrInvalidDataLength), 6},
		{"request too large", ExitRuntime(client.ErrRequestExceedsPDU), 3},
		{"permission", ExitConnect(fs.ErrPermission), 9},
		{"output file", ExitRuntime(&fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist}), 10},
	} {
		if got := exitCode(c.err); got != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, got, c.want)
		}
	}
	for _, f := range []func(error) error{ExitUsage, ExitOperation, ExitConnect, ExitRuntime} {
		if f(nil) != nil {
			t.Error("nil error must stay nil")
		}
	}
}
