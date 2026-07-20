// SPDX-License-Identifier: MIT

package s7commctl

import (
	"strings"
	"testing"
)

func TestConfirmWrite(t *testing.T) {
	if !confirmWrite(strings.NewReader("yes\n")) {
		t.Fatal("expected yes confirmation")
	}
	if confirmWrite(strings.NewReader("no\n")) {
		t.Fatal("expected no confirmation to fail")
	}
}
