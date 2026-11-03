// SPDX-License-Identifier: MIT

package s7commctl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReplayScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.txt")
	content := "# comment\n0300000702F080\n03 00 00 07 02 F0 80\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	frames, err := loadReplayScript(path)
	if err != nil {
		t.Fatalf("loadReplayScript failed: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	if len(frames[0]) != 7 {
		t.Fatalf("unexpected frame length %d", len(frames[0]))
	}
}

// FuzzLoadReplayScript: a script either loads or is refused, and every loaded
// response is a TPKT frame start.
func FuzzLoadReplayScript(f *testing.F) {
	f.Add("# comment\n0300000702F080\n03 00 00 07 02 F0 80\n")
	f.Add("zz\n")
	f.Add("0400000702F080\n")
	f.Fuzz(func(t *testing.T, content string) {
		path := filepath.Join(t.TempDir(), "script.txt")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		frames, err := loadReplayScript(path)
		if err != nil {
			return
		}
		for _, b := range frames {
			if len(b) < 4 || b[0] != 0x03 {
				t.Fatalf("loaded a non-TPKT response % X", b)
			}
		}
	})
}
