package editor

import (
	"strings"
	"testing"
)

func TestWrapDoesNotExceedColumnWidth(t *testing.T) {
	for _, line := range wrap("bu prompt yatay kaydırma gerektirmeden okunabilir olmalı", 12) {
		if len([]rune(line)) > 12 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}

func TestReplaceCodexBlockUsesAbsoluteEditor(t *testing.T) {
	got := replaceCodexBlock("before\n# promptcheck Codex editor\ncodex() { old; }\nafter\n", codexBlock("/opt/bin/promptcheck"))
	if !strings.Contains(got, "VISUAL='/opt/bin/promptcheck edit'") || strings.Contains(got, "old") {
		t.Fatalf("block=%q", got)
	}
}
