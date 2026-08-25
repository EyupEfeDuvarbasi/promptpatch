package quality

import (
	"strings"
	"testing"
)

func TestRewriteIssuesRejectsIncompleteGenericAndInventedOutput(t *testing.T) {
	original := "PromptPatch ile PromptLens için uygulanabilir bir birleşim planı oluştur."
	candidate := "Bu, oldukça karmaşık bir projedir. Başarılı olmak için 2 hafta boyunca aşağıdaki adımları uygula:\n\n## Plan\n- PromptPatch ile PromptLens'i birleştir\n-"
	issues := strings.Join(RewriteIssues(original, original, candidate, []string{"PromptPatch", "PromptLens"}), " | ")
	for _, want := range []string{"genel bir giriş", "2 hafta", "tamamlanmamış"} {
		if !strings.Contains(issues, want) {
			t.Fatalf("issues=%q, missing %q", issues, want)
		}
	}
}

func TestRewriteIssuesAllowsNecessaryDetailWithoutLengthRule(t *testing.T) {
	original := "PromptPatch ile PromptLens'i birleştirmek için eksiksiz bir plan oluştur."
	candidate := "PromptPatch ile PromptLens'i tek ürün hâline getirecek uygulanabilir bir plan hazırla. Mevcut mimarileri ve veri akışlarını karşılaştır; ortak kullanıcı yolculuğunu, entegrasyon sınırlarını, API sözleşmesini, hata davranışlarını, güvenlik gereksinimlerini, test stratejisini ve aşamalı geçiş sırasını açıkla. Her aşama için doğrulanabilir kabul kriterleri belirt ve belirsiz kalan kritik kararları ayrıca listele."
	if issues := RewriteIssues(original, original, candidate, []string{"PromptPatch", "PromptLens"}); len(issues) != 0 {
		t.Fatalf("issues=%q", issues)
	}
}

func TestRewriteIssuesAcceptsFileAndLineWrittenSeparately(t *testing.T) {
	original := "Stack trace `internal/payment/worker.go:87` satırını gösteriyor."
	candidate := "`internal/payment/worker.go` dosyasının `87` satırındaki hatayı düzelt."
	if issues := RewriteIssues(original, original, candidate, []string{"internal/payment/worker.go:87"}); len(issues) != 0 {
		t.Fatalf("issues=%q", issues)
	}
}
