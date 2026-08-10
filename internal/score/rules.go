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

var ambiguousTerms = []string{"şunu", "bunu", "bir şekilde", "falan filan", "vs.", "vesaire"}
var contextTerms = []string{
	".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".rb", ".php",
	"fonksiyon", "function", "func", "metot", "method", "sınıf", "class",
	"bağlam", "hata", "error", "stack trace", "react", "go", "golang", "python", "node", "typescript", "javascript", "rust", "java",
}
var constraintTerms = []string{"yalnızca", "sadece", "değiştirme", "koru", "olmamalı", "kapsam", "must", "only", "without", "do not"}
var outputTerms = []string{"çıktı", "format", "beklenen sonuç", "kabul kriterleri", "json", "markdown", "tablo", "liste", "test", "örnek", "example", "dön", "olmalı", "oluşmasın", "doğrula"}
var actionTerms = []string{"düzelt", "ekle", "oluştur", "güncelle", "sil", "sağla", "refactor", "fix", "add", "create", "update", "remove"}

// Evaluate scores observable prompt signals. Semantic quality belongs to the LLM layer.
func Evaluate(prompt string) Result {
	text := strings.ToLower(strings.TrimSpace(prompt))
	words := strings.Fields(text)
	findings := []string{}

	action := matches(text, actionTerms) > 0
	ambiguous := matches(text, ambiguousTerms) > 0
	contextSignals := matches(text, contextTerms)
	outputSignals := matches(text, outputTerms)
	constraintSignals := matches(text, constraintTerms)

	clarity := 25
	if action {
		clarity = 70
	}
	if len(words) >= 5 && !ambiguous {
		clarity += 30
	}
	if ambiguous {
		clarity -= 40
		findings = append(findings, "AI'ın tahmin yapmasına yol açan belirsiz ifade var.")
	}
	if !action {
		findings = append(findings, "Yapılacak görev açıkça belirtilmemiş.")
	}

	context := signalScore(contextSignals)
	if contextSignals == 0 {
		findings = append(findings, "Dosya, bileşen, teknoloji veya mevcut durum bağlamı belirtilmemiş.")
	}

	expected := signalScore(outputSignals)
	if outputSignals == 0 {
		findings = append(findings, "Beklenen davranış ya da kabul ölçütü belirtilmemiş.")
	}

	constraints := signalScore(constraintSignals)
	if constraintSignals == 0 {
		findings = append(findings, "Kapsam veya korunacak sınırlar belirtilmemiş.")
	}

	applicability := 100
	if !action {
		applicability -= 25
	}
	if ambiguous {
		applicability -= 35
	}
	if contextSignals == 0 {
		applicability -= 20
	}
	if outputSignals == 0 {
		applicability -= 20
	}

	criteria := []Criterion{
		{Name: "Amaç ve Görev Netliği", Score: clamp(clarity)},
		{Name: "Bağlam ve Teknik Bilgi", Score: context},
		{Name: "Beklenen Sonuç", Score: expected},
		{Name: "Kısıtlar ve Sınırlar", Score: constraints},
		{Name: "Belirsizlik / Uygulanabilirlik", Score: clamp(applicability)},
	}
	return Result{
		Criteria:        criteria,
		Findings:        findings,
		Score:           average(criteria),
		NeedsContext:    contextSignals == 0,
		NeedsFormat:     outputSignals == 0,
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
