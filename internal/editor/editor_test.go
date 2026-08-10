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
	got := replaceCodexBlock("before\n# promptcheck Codex editor\ncodex() { old; }\nafter\n", codexBlock("/opt/bin/promptpatch-codex-editor"))
	if !strings.Contains(got, "VISUAL='/opt/bin/promptpatch-codex-editor'") || strings.Contains(got, "old") {
		t.Fatalf("block=%q", got)
	}
}

func TestWrapperPassesEditorFile(t *testing.T) {
	if got := wrapperScript("/opt/bin/promptcheck"); got != "#!/bin/sh\nexec '/opt/bin/promptcheck' edit \"$@\"\n" {
		t.Fatalf("script=%q", got)
	}
}

func TestScreenLinesUseCarriageReturn(t *testing.T) {
	if got := screenText("a\nb"); got != "a\r\nb" {
		t.Fatalf("line ending=%q", got)
	}
}

func TestRemoveLastRunePreservesTurkishText(t *testing.T) {
	if got := string(removeLastRune([]byte("çerez"))); got != "çere" {
		t.Fatalf("answer=%q", got)
	}
}

func TestUsableRewriteRequiresAnsweredContext(t *testing.T) {
	answers := []string{"src/parser.go", "Boş girdi hata dönsün"}
	if !usableRewrite("src/parser.go dosyasındaki parseInput fonksiyonunda boş girdi için hata dönsün.", answers) {
		t.Fatal("expected rewrite to retain answers")
	}
	if usableRewrite("Kod kalitesini artır ve gerekli düzeltmeleri yap.", answers) {
		t.Fatal("rewrite dropped answered context")
	}
}
