package editor

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestWrapDoesNotExceedColumnWidth(t *testing.T) {
	for _, line := range wrap("bu prompt yatay kaydırma gerektirmeden okunabilir olmalı", 12) {
		if len([]rune(line)) > 12 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}

func TestReadKeyAcceptsLoneEscape(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	previous := terminalInput
	terminalInput = reader
	defer func() { terminalInput = previous }()
	if _, err := writer.Write([]byte{27}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if got := readKey(); got != "esc" || time.Since(started) > 200*time.Millisecond {
		t.Fatalf("key=%q elapsed=%s", got, time.Since(started))
	}
}

func TestReplaceCodexBlockUsesAbsoluteEditor(t *testing.T) {
	got := replaceCodexBlock("before\n# promptcheck Codex editor\ncodex() { old; }\nafter\n", codexBlock("/opt/bin/promptpatch-codex-editor"))
	if !strings.Contains(got, "PROMPTPATCH_HOST=codex VISUAL='/opt/bin/promptpatch-codex-editor'") || strings.Contains(got, "old") {
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

func TestUsableRewriteRejectsRawQA(t *testing.T) {
	if !usableRewrite("şunu düzelt", "src/parser.go dosyasındaki parseInput fonksiyonunda boş girdi için hata dönsün.") {
		t.Fatal("expected usable rewrite")
	}
	if usableRewrite("şunu düzelt", "Soru: Hangi dosya? Cevap: src/parser.go") {
		t.Fatal("raw Q&A must be rejected")
	}
}

func TestUsableRewriteAllowsStructuredPrompt(t *testing.T) {
	candidate := "## Görev\nsrc/parser.go içindeki parseInput fonksiyonunda boş girdi hatasını düzelt.\n\n## Başarı ölçütleri\nBoş girdi hata döndürmeli."
	if !usableRewrite("şunu düzelt", candidate) {
		t.Fatal("structured prompt should be usable")
	}
}

func TestScoreSummaryIsDecisionFocused(t *testing.T) {
	if scoreBadge(64) != "64/100" || scoreDelta(15) != "  (+15)" || scoreDelta(-4) != "  (-4)" || scoreDelta(0) != "  (aynı)" {
		t.Fatal("score summary is not stable")
	}
}

func TestPromptPreviewTruncatesToTerminalBudget(t *testing.T) {
	preview := promptPreview("bir iki üç dört beş altı yedi sekiz dokuz on", 8, 3)
	if len(preview) != 3 || !strings.HasPrefix(preview[2], "… (") {
		t.Fatalf("preview=%q", preview)
	}
}
