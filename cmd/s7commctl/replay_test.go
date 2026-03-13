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
