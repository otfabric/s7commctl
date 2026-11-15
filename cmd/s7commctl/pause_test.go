// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestSleepCtx(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		if err := sleepCtx(context.Background(), time.Second); err != nil {
			t.Fatalf("err = %v", err)
		}
		if d := time.Since(start); d != time.Second {
			t.Fatalf("waited %v, want 1s", d)
		}

		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(100*time.Millisecond, cancel)
		start = time.Now()
		if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if d := time.Since(start); d != 100*time.Millisecond {
			t.Fatalf("waited %v after the cancel at 100ms", d)
		}
	})
}

// interruptPause makes every pause of the command an interrupt: it cancels
// the run's context, then waits as the command would.
func interruptPause(t *testing.T, cancel context.CancelFunc) {
	t.Helper()
	wait := pause
	pause = func(ctx context.Context, d time.Duration) error {
		cancel()
		return wait(ctx, d)
	}
	t.Cleanup(func() { pause = wait })
}

// interruptedPromptly asserts the outcome of a command interrupted in its
// pause of interval: exit 130, nothing on stdout, and no wait for the pause.
func interruptedPromptly(t *testing.T, r cliResult, start time.Time, interval time.Duration) {
	t.Helper()
	wantInterrupted(t, r)
	if r.stdout != "" {
		t.Errorf("an interrupted run printed a result:\n%s", r.stdout)
	}
	if d := time.Since(start); d >= interval {
		t.Errorf("returned after %v: the pause of %v was not interrupted", d, interval)
	}
}

func TestCompare_InterruptDuringTheInterval(t *testing.T) {
	p := newFakePLC(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interruptPause(t, cancel)
	start := time.Now()
	r := runCLIContext(t, ctx, compareArgs(p, "--repeat", "2", "--interval-ms", "2000")...)
	interruptedPromptly(t, r, start, 2*time.Second)
}

// TestCompare_InterruptedRunIsNoResult: a run the interrupt cut short is not
// reported as the result.
func TestCompare_InterruptedRunIsNoResult(t *testing.T) {
	p := newFakePLC(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.read = func(_ byte, _, start, size int) fakeReply {
		cancel()
		return fakeReply{data: patternBytes(start, size)}
	}
	for _, extra := range [][]string{nil, {"--format", "json"}} {
		r := runCLIContext(t, ctx, compareArgs(p, extra...)...)
		wantInterrupted(t, r)
		if r.stdout != "" {
			t.Errorf("%v: an interrupted run printed a result:\n%s", extra, r.stdout)
		}
	}
}

func TestProbe_InterruptDuringTheInterval(t *testing.T) {
	p := probePLC(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interruptPause(t, cancel)
	start := time.Now()
	r := runCLIContext(t, ctx, probeArgs(p, "--repeat", "2", "--interval-ms", "2000")...)
	interruptedPromptly(t, r, start, 2*time.Second)
}

func TestReplay_InterruptDuringTheDelay(t *testing.T) {
	script := writeReplayScript(t, tpktFrame([]byte{0x02, 0xF0, 0x80, 0xAA}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interruptPause(t, cancel)
	addr := freeAddr(t)
	start := time.Now()
	conn, wait := startServerContext(t, ctx, addr, "replay", "--listen", addr, "--script", script, "--delay-ms", "2000")
	if _, err := conn.Write(s7ReadRequestFrame(1)); err != nil {
		t.Fatal(err)
	}
	r := wait()
	wantInterrupted(t, r)
	if d := time.Since(start); d >= 2*time.Second {
		t.Errorf("returned after %v: the delay of 2s was not interrupted", d)
	}
}
