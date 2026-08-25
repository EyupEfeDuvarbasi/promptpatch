// Package quality validates generated prompt rewrites.
package quality

import (
	"regexp"
	"strings"
	"unicode"
)

var numericClaim = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:ms|sn|saniye|dakika|saat|gün|hafta|ay|yıl|kb|mb|gb|tb|fps|hz|%)\b`)

// RewriteIssues returns concise reasons why candidate is unsafe to apply.
func RewriteIssues(original, source, candidate string, required []string) []string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return []string{"iyileştirilmiş prompt boş"}
	}
	issues := []string{}
	if normalize(original) == normalize(candidate) {
		issues = append(issues, "özgün prompt yeniden yazılmadı")
	}
	lower := normalize(candidate)
	if strings.Contains(lower, "soru:") || strings.Contains(lower, "cevap:") {
		issues = append(issues, "soru-cevap metni çıktıya sızdı")
	}
	if strings.Contains(lower, "zorunlu ifadeler") || strings.Contains(lower, "required_terms") || strings.Contains(lower, "confirmed_answers") {
		issues = append(issues, "üretim metadatası çıktıya sızdı")
	}
	for _, prefix := range []string{
		"bu oldukça karmaşık", "bu, oldukça karmaşık", "başarılı olmak için",
		"işte daha iyi", "işte iyileştirilmiş", "elbette", "tabii ki",
	} {
		if strings.HasPrefix(lower, prefix) {
			issues = append(issues, "genel bir giriş yerine doğrudan görev yazılmalı")
			break
		}
	}
	for _, claim := range []string{"önceden belirlenen adımlar", "mevcut sorunları çöz", "sorunlarını çöz"} {
		if strings.Contains(lower, claim) && !strings.Contains(normalize(source), "sorun") && !strings.Contains(normalize(source), claim) {
			issues = append(issues, "kaynakta olmayan bir durum gerçekmiş gibi yazıldı")
			break
		}
	}
	for _, fact := range required {
		if fact = strings.TrimSpace(fact); fact != "" && !factPresent(lower, normalize(fact)) {
			issues = append(issues, "somut gereksinim korunmadı: "+fact)
		}
	}
	allowed := normalize(source + "\n" + original)
	for _, claim := range numericClaim.FindAllString(candidate, -1) {
		if !strings.Contains(allowed, normalize(claim)) {
			issues = append(issues, "kaynakta olmayan sayısal bilgi eklendi: "+strings.TrimSpace(claim))
		}
	}
	if strings.Count(candidate, "```")%2 != 0 || unfinishedEnding(candidate) {
		issues = append(issues, "çıktı tamamlanmamış görünüyor")
	}
	if repeatedLine(candidate) {
		issues = append(issues, "aynı bölüm veya cümle tekrarlandı")
	}
	return unique(issues)
}

func factPresent(candidate, fact string) bool {
	if strings.Contains(candidate, fact) {
		return true
	}
	if index := strings.LastIndex(fact, ":"); index > 0 && allDigits(fact[index+1:]) {
		return strings.Contains(candidate, fact[:index]) && strings.Contains(candidate, fact[index+1:])
	}
	if strings.HasPrefix(fact, "md dosyası") {
		return strings.Contains(candidate, "markdown format") || strings.Contains(candidate, "markdown biçim") || strings.Contains(candidate, "markdown dosya")
	}
	return false
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}

func normalize(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func unfinishedEnding(value string) bool {
	last := strings.TrimSpace(value)
	if last == "" {
		return true
	}
	line := strings.TrimSpace(last[strings.LastIndex(last, "\n")+1:])
	if line == "-" || line == "*" || line == "+" || strings.HasSuffix(line, "##") {
		return true
	}
	fields := strings.Fields(strings.TrimRightFunc(line, unicode.IsPunct))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[len(fields)-1]) {
	case "ve", "veya", "ile", "için", "ancak", "fakat", "çünkü":
		return true
	}
	return false
}

func repeatedLine(value string) bool {
	seen := map[string]bool{}
	for _, line := range strings.Split(value, "\n") {
		line = normalize(strings.TrimLeft(strings.TrimSpace(line), "#-*+0123456789. "))
		if len([]rune(line)) < 24 {
			continue
		}
		if seen[line] {
			return true
		}
		seen[line] = true
	}
	return false
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
