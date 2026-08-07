// Package score provides the local, rule-based portion of prompt evaluation.
package score

import (
	"strings"
)

type Criterion struct {
	Name  string
	Score int
}

type Result struct {
	Criteria        []Criterion
	Findings        []string
	Score           int
	NeedsContext    bool
	NeedsFormat     bool
	NeedsClarifying bool
}

var ambiguousTerms = []string{"şunu", "bunu", "bir şekilde", "falan filan"}
var contextTerms = []string{
	".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".rb", ".php",
	"fonksiyon", "function", "func", "metot", "method", "sınıf", "class",
	"react", "go", "golang", "python", "node", "typescript", "javascript", "rust", "java",
}
var constraintTerms = []string{"yalnızca", "sadece", "değiştirme", "koru", "olmamalı", "must", "only", "without", "do not"}
var outputTerms = []string{"çıktı", "format", "json", "markdown", "tablo", "liste", "test", "örnek", "example"}
var purposeTerms = []string{"çünkü", "amac", "amaç", "böylece", "için", "so that", "because"}
var actionTerms = []string{"düzelt", "ekle", "oluştur", "güncelle", "sil", "refactor", "fix", "add", "create", "update", "remove"}

// Evaluate scores observable prompt signals. Semantic quality belongs to the LLM layer.
func Evaluate(prompt string) Result {
	text := strings.ToLower(strings.TrimSpace(prompt))
	words := strings.Fields(text)
	findings := []string{}

	clarity := 100
	if len(words) < 5 {
		clarity -= 55
		findings = append(findings, "Prompt beş kelimeden kısa.")
	} else if len(words) < 10 {
		clarity -= 25
	}
	if ambiguous := matches(text, ambiguousTerms); ambiguous > 0 {
		clarity -= ambiguous * 25
		findings = append(findings, "Belirsiz ifade tespit edildi.")
	}

	contextSignals := matches(text, contextTerms)
	specificity := signalScore(contextSignals)
	context := signalScore(contextSignals)
	if contextSignals == 0 {
		findings = append(findings, "Dosya, fonksiyon veya teknoloji bağlamı belirtilmemiş.")
	}

	constraints := matches(text, constraintTerms)
	outputs := matches(text, outputTerms)
	format := 30
	if constraints > 0 {
		format += 35
	}
	if outputs > 0 {
		format += 35
	}
	if constraints == 0 && outputs == 0 {
		findings = append(findings, "Kısıt veya beklenen çıktı formatı belirtilmemiş.")
	}

	purpose := 35
	if matches(text, actionTerms) > 0 {
		purpose = 65
	}
	if matches(text, purposeTerms) > 0 {
		purpose = 100
	}

	criteria := []Criterion{
		{Name: "Netlik", Score: clamp(clarity)},
		{Name: "Spesifiklik", Score: specificity},
		{Name: "Bağlam Yeterliliği", Score: context},
		{Name: "Kısıtlar ve Çıktı", Score: format},
		{Name: "Amaç ve Başarı Ölçütü", Score: purpose},
	}
	return Result{
		Criteria:        criteria,
		Findings:        findings,
		Score:           average(criteria),
		NeedsContext:    contextSignals == 0,
		NeedsFormat:     constraints == 0 && outputs == 0,
		NeedsClarifying: len(words) < 5 || matches(text, ambiguousTerms) > 0,
	}
}

func matches(text string, terms []string) int {
	count := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			count++
		}
	}
	return count
}

func signalScore(signals int) int {
	switch {
	case signals == 0:
		return 30
	case signals == 1:
		return 65
	default:
		return 100
	}
}

func clamp(score int) int {
	return max(0, min(100, score))
}

func average(criteria []Criterion) int {
	total := 0
	for _, criterion := range criteria {
		total += criterion.Score
	}
	return total / len(criteria)
}
