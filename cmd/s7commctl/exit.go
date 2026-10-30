// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/wire"
)

// Process exit codes: the organisation table (docs/exitcodes.md).
const (
	codeOK           = 0
	codeFindings     = 1 // the operation completed and reports a non-success
	codeUsage        = 2
	codeInvalidInput = 3
	codeTransport    = 4
	codeTimeout      = 5
	codeProtocol     = 6
	codePermission   = 9
	codeInternal     = 10
	codeInterrupted  = 130 // 128 + SIGINT, as a shell reports it
)

// ExitError carries a process exit code and an error for Execute() to handle.
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

// ExitUsage returns an error that will cause exit code 2 (usage, validation,
// missing confirmation).
func ExitUsage(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: codeUsage, Err: err}
}

// ExitOperation returns an error that will cause exit code 1: the operation
// completed and its result is a non-success.
func ExitOperation(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: codeFindings, Err: err}
}

// ExitConnect returns an error for a failed connection attempt: exit code 4,
// or what classify finds in err.
func ExitConnect(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: classify(err, codeTransport), Err: err}
}

// ExitRuntime returns an error for an operation that failed on the PLC or the
// connection: exit code 6, or what classify finds in err.
func ExitRuntime(err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: classify(err, codeProtocol), Err: err}
}

// classify returns the exit code of err, def when nothing in it is specific.
func classify(err error, def int) int {
	var (
		ne net.Error
		oe *net.OpError
		pe *fs.PathError
	)
	switch {
	case errors.Is(err, context.Canceled):
		return codeInterrupted
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return codeTimeout
	case errors.Is(err, fs.ErrPermission):
		return codePermission
	case errors.Is(err, client.ErrRequestExceedsPDU):
		return codeInvalidInput
	case errors.Is(err, client.ErrProtocolFailure), errors.Is(err, wire.ErrTruncatedItemHeader),
		errors.Is(err, wire.ErrTruncatedItemPayload), errors.Is(err, wire.ErrInvalidParamLength),
		errors.Is(err, wire.ErrInvalidDataLength):
		return codeProtocol
	case errors.Is(err, client.ErrNotConnected), errors.As(err, &oe), errors.As(err, &ne):
		return codeTransport
	case errors.As(err, &pe):
		return codeInternal
	}
	return def
}

// exitCode maps the error of a run to the process exit code. An error that
// is no ExitError comes from cobra: the command line was rejected.
func exitCode(err error) int {
	if err == nil {
		return codeOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return codeUsage
}
