// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"time"
)

// sleepCtx waits d, or until ctx ends, and returns ctx.Err() when ctx ended
// first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// pause is the wait between --repeat runs and before a replayed response, so
// that an interrupt ends it. Tests replace it to interrupt at that point.
var pause = sleepCtx
