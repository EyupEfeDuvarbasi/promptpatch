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

func TestRunWorksWithoutAPIKey(t *testing.T) {
	var output bytes.Buffer
	err := Run(context.Background(), []string{"şunu düzelt"}, strings.NewReader("src/parser.go\nJSON hata açıklaması\n"), &output, nil)
	if err != nil || !strings.Contains(output.String(), "İyileştirilmiş Prompt") {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
}
