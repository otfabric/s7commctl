// SPDX-License-Identifier: MIT

package s7commctl

import (
	"errors"
	"testing"
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
	if ee.Code != 3 {
		t.Errorf("expected code 3 (runtime), got %d", ee.Code)
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
