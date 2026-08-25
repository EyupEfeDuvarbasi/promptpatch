package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

func TestReadPrompt(t *testing.T) {
	prompt, err := readPrompt(nil, strings.NewReader("  parserı düzelt  \n"))
	if err != nil || prompt != "parserı düzelt" {
		t.Fatalf("prompt=%q err=%v", prompt, err)
	}
}

func TestBlendUsesConfiguredWeights(t *testing.T) {
	rules := score.Result{Criteria: []score.Criterion{{Name: "Netlik", Score: 50}}}
	result := blend(rules, []score.Criterion{{Name: "Netlik", Score: 100}})
	if result.Score != 80 || result.Criteria[0].Score != 80 {
		t.Fatalf("result=%#v", result)
	}
}

func TestBlendMatchesCriteriaByName(t *testing.T) {
	rules := score.Result{Criteria: []score.Criterion{{Name: "A", Score: 50}, {Name: "B", Score: 60}}}
	result := blend(rules, []score.Criterion{{Name: "B", Score: 100}, {Name: "A", Score: 0}})
	if result.Criteria[0].Score != 20 || result.Criteria[1].Score != 84 {
		t.Fatalf("result=%#v", result)
	}
}

func TestRunPrintsHelp(t *testing.T) {
	var output bytes.Buffer
	if err := Run(context.Background(), []string{"--help"}, strings.NewReader(""), &output, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"setup-codex", "edit <dosya>", "Ctrl-G"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("help=%q, missing %q", output.String(), want)
		}
	}
}

func TestLocalQuestionsAreBounded(t *testing.T) {
	questions := LocalQuestions(score.Result{NeedsContext: true, NeedsFormat: true, NeedsClarifying: true})
	if len(questions) != 1 {
		t.Fatalf("questions=%v", questions)
	}
}

func TestLocalQuestionsUseConversationContext(t *testing.T) {
	result := score.Evaluate("şunu düzelt")
	context := "USER: src/parser.go içindeki parseInput fonksiyonunda boş girdi panic ediyor. Mevcut davranışı koru ve birim test ekle."
	questions := LocalQuestionsWithContext(result, "şunu düzelt", context)
	if len(questions) != 0 {
		t.Fatalf("context already contains task details, questions=%v", questions)
	}
}

func TestLocalQuestionsAskOnlyMissingOutputWithContext(t *testing.T) {
	result := score.Evaluate("şunu düzelt")
	context := "USER: src/parser.go içindeki parseInput fonksiyonundaki hatayı düzelt."
	questions := LocalQuestionsWithContext(result, "şunu düzelt", context)
	if len(questions) != 1 || !strings.Contains(questions[0], "çıktı") {
		t.Fatalf("questions=%v", questions)
	}
}

func TestLocalQuestionsAdaptToTaskType(t *testing.T) {
	questions := LocalQuestions(score.Result{Kind: score.Performance, NeedsContext: true, NeedsFormat: true})
	if len(questions) != 1 || !strings.Contains(questions[0], "iş yükü") {
		t.Fatalf("questions=%q", questions)
	}
}

func TestLocalImproveLabelsOnlyProvidedAnswers(t *testing.T) {
	got := LocalImprove("şunu düzelt", []string{"Hangi dosya?", "Beklenen sonuç?"}, []string{"src/parser.go", "boş girdi hata dönsün"})
	if !strings.Contains(got, "# Hata düzeltme\n\n## Amaç\nşunu düzelt") || !strings.Contains(got, "## Bağlam\n- src/parser.go") || !strings.Contains(got, "## Beklenen sonuç\n- boş girdi hata dönsün") || strings.Contains(got, "Hangi dosya?") {
		t.Fatalf("improved=%q", got)
	}
}

func TestLocalImprovePreservesAcceptanceCriteria(t *testing.T) {
	got := LocalImprove("footer ekle\nKabul kriterleri: tüm linkler erişilebilir olmalı", nil, nil)
	if !strings.Contains(got, "# Özellik geliştirme") || !strings.Contains(got, "## Kabul kriterleri\ntüm linkler erişilebilir olmalı") {
		t.Fatalf("improved=%q", got)
	}
}

func TestLocalImproveRaisesStructuralScore(t *testing.T) {
	original := "şunu düzelt"
	improved := LocalImprove(original, []string{"Hangi dosya?", "Beklenen sonuç?"}, []string{"src/parser.go", "boş girdi hata dönsün"})
	if score.Evaluate(improved).Score <= score.Evaluate(original).Score {
		t.Fatalf("scores: original=%d improved=%d", score.Evaluate(original).Score, score.Evaluate(improved).Score)
	}
}

func TestLocalImproveWithContextPreservesLatestUserTask(t *testing.T) {
	got := LocalImproveWithContext("şunu düzelt", "ASSISTANT: Önce bağlamı kontrol et.\n\nUSER: src/parser.go içindeki parseInput fonksiyonu boş girdide panic ediyor. Birim test ekle.", nil, nil)
	if !strings.Contains(got, "## Bağlam") || !strings.Contains(got, "src/parser.go") || !strings.Contains(got, "parseInput") {
		t.Fatalf("improved=%q", got)
	}
}

func TestRunRejectsDirectPromptInput(t *testing.T) {
	var output bytes.Buffer
	err := Run(context.Background(), []string{"şunu düzelt"}, strings.NewReader("src/parser.go\nJSON hata açıklaması\n"), &output, nil)
	if err == nil || !strings.Contains(err.Error(), "Ctrl-G") {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
}
