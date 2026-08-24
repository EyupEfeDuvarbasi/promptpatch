package main

import (
	"fmt"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

type sample struct {
	prompt  string
	context string
}

func answerFor(question string) string {
	q := strings.ToLower(question)
	switch {
	case strings.Contains(q, "dosya") || strings.Contains(q, "bileşen") || strings.Contains(q, "endpoint"):
		return "src/parser.go içindeki parseInput fonksiyonu"
	case strings.Contains(q, "iş yükü") || strings.Contains(q, "ölçüm"):
		return "GET /search için mevcut p95 800 ms; hedef p95 250 ms altı"
	case strings.Contains(q, "format") || strings.Contains(q, "çıktı") || strings.Contains(q, "davranış"):
		return "Mevcut davranışı koru, hatayı açıkça döndür ve birim test ekle"
	case strings.Contains(q, "plan"):
		return "Kısa Markdown planı; amaç, risk, doğrulama ve görünür çıktı bölümleri olsun"
	default:
		return "Kullanıcının mevcut davranışını bozma ve değişikliği testlerle doğrula"
	}
}

func main() {
	samples := []sample{
		{prompt: "şunu düzelt"},
		{prompt: "kullanıcı profili ekle"},
		{prompt: "uygulamayı hızlandır"},
		{prompt: "auth güvenliğini artır"},
		{prompt: "davet API'si yap"},
		{prompt: "buna test yaz"},
		{prompt: "dokümantasyonu düzelt"},
		{prompt: "paneli daha güzel yap"},
		{prompt: "isim alanını ikiye böl"},
		{
			prompt:  "şunu düzelt",
			context: "USER: src/parser.go içindeki parseInput fonksiyonu boş girdide panic ediyor. Mevcut davranışı koru ve birim test ekle.",
		},
	}

	for i, sample := range samples {
		original := score.Evaluate(sample.prompt)
		questions := cli.LocalQuestionsWithContext(original, sample.prompt, sample.context)
		answers := make([]string, len(questions))
		for j, question := range questions {
			answers[j] = answerFor(question)
		}
		improved := cli.LocalImproveWithContext(sample.prompt, sample.context, questions, answers)
		improvedResult := score.Evaluate(improved)

		fmt.Printf("\n===== %d =====\n", i+1)
		fmt.Printf("Özgün: %s\n", sample.prompt)
		fmt.Printf("Özgün skor: %d/100\n", original.Score)
		if sample.context != "" {
			fmt.Printf("Bağlam: %s\n", sample.context)
		}
		fmt.Printf("Sorular: %v\n", questions)
		fmt.Printf("Cevaplar: %v\n", answers)
		fmt.Printf("İyileştirilmiş skor: %d/100\n", improvedResult.Score)
		fmt.Printf("İyileştirilmiş:\n%s\n", improved)
	}
}
