// SPDX-License-Identifier: MIT

package s7commctl

import "fmt"

// ExitError carries a process exit code and an error for Execute() to handle.
// Code semantics (see README): 0 success, 1 target/operation non-success, 2 usage/validation, 3 connection/runtime.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func (e *ExitError) Unwrap() error { return e.Err }

// ExitUsage returns an error that will cause exit code 2 (usage/validation).
func ExitUsage(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: 2, Err: err}
}

// ExitOperation returns an error that will cause exit code 1 (target/operation non-success).
func ExitOperation(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: 1, Err: err}
}

// ExitRuntime returns an error that will cause exit code 3 (connection/runtime failure).
func ExitRuntime(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: 3, Err: err}
}
