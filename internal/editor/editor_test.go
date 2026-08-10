package editor

import (
	"strings"
	"testing"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
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

func TestUsableRewriteRejectsRawQA(t *testing.T) {
	if !usableRewrite("src/parser.go dosyasındaki parseInput fonksiyonunda boş girdi için hata dönsün.") {
		t.Fatal("expected usable rewrite")
	}
	if usableRewrite("Soru: Hangi dosya? Cevap: src/parser.go") {
		t.Fatal("raw Q&A must be rejected")
	}
}

func TestPlainFallbackHasNoMarkdownHeadings(t *testing.T) {
	got := plainFallback("şunu düzelt", []string{"Hangi dosya?", "Beklenen sonuç?"}, []string{"src/parser.go", "Boş girdi hata dönsün"})
	if strings.Contains(got, "#") || !strings.Contains(got, "Bağlam: src/parser.go") || !strings.Contains(got, "Beklenen sonuç: Boş girdi hata dönsün") {
		t.Fatalf("fallback=%q", got)
	}
}

func TestScoreTitleIsCompact(t *testing.T) {
	result := score.Result{Score: 64, Criteria: []score.Criterion{{Score: 70}, {Score: 60}, {Score: 50}, {Score: 40}, {Score: 100}}}
	if got := scoreTitle("Özgün prompt", result); got != "Özgün prompt — 64/100 | G:70 B:60 S:50 K:40 U:100" {
		t.Fatalf("title=%q", got)
	}
}

func TestPromptPreviewTruncatesToTerminalBudget(t *testing.T) {
	preview := promptPreview("bir iki üç dört beş altı yedi sekiz dokuz on", 8, 3)
	if len(preview) != 3 || !strings.HasPrefix(preview[2], "… (") {
		t.Fatalf("preview=%q", preview)
	}
}
