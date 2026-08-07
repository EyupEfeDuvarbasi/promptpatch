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

func TestLocalQuestionsAreLimited(t *testing.T) {
	questions := LocalQuestions(score.Result{NeedsContext: true, NeedsFormat: true, NeedsClarifying: true})
	if len(questions) != 2 {
		t.Fatalf("questions=%v", questions)
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

func TestRunWorksWithoutAPIKey(t *testing.T) {
	var output bytes.Buffer
	err := Run(context.Background(), []string{"şunu düzelt"}, strings.NewReader("src/parser.go\nJSON hata açıklaması\n"), &output, nil)
	if err != nil || !strings.Contains(output.String(), "İyileştirilmiş Prompt") {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
}
