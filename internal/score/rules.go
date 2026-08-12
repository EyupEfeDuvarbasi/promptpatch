// Package score provides deterministic, local prompt evaluation.
package score

import "strings"

type Criterion struct {
	Name  string
	Score int
}

type TaskKind string

const (
	General       TaskKind = "general"
	BugFix        TaskKind = "bug_fix"
	Feature       TaskKind = "feature"
	Refactor      TaskKind = "refactor"
	Performance   TaskKind = "performance"
	ResearchPlan  TaskKind = "research_plan"
	APIContract   TaskKind = "api_contract"
	Security      TaskKind = "security"
	DataMigration TaskKind = "data_migration"
	Testing       TaskKind = "testing"
	Documentation TaskKind = "documentation"
	Review        TaskKind = "review"
	Content       TaskKind = "content"
)

type Result struct {
	Criteria        []Criterion
	Findings        []string
	Score           int
	Kind            TaskKind
	NeedsContext    bool
	NeedsFormat     bool
	NeedsClarifying bool
	Contradictory   bool
}

var vagueTerms = []string{"şunu", "bunu", "bir şekilde", "falan filan", "vs.", "vesaire", "hızlandır", "en iyi şekilde", "daha güzel", "geliştir", "fix the bug", "fix bug", "direk", "direkt"}
var actionTerms = []string{"düzelt", "fix", "ekle", "add", "oluştur", "create", "güncelle", "update", "sil", "remove", "sağla", "sagla", "indir", "düşür", "dusur", "refactor", "incele", "araştır", "araştırma", "plan", "yaz", "çevir", "özetle", "açıkla", "migrate", "migration"}
var contextTerms = []string{".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", "fonksiyon", "function", "func", "metot", "method", "endpoint", "api", "url", "hata", "error", "panic", "stack trace", "react", "go", "golang", "python", "node", "typescript", "javascript", "rust", "java", "jetson", "kamera", "database", "veritaban", "tablo", "component", "bileşen", "ekran", "ui", "sayfa", "klavye", "terminal", "cli", "komut", "help", "readme"}
var outputTerms = []string{"çıktı", "format", "beklenen", "kabul kriter", "json", "markdown", "md dosyası", "tablo", "liste", "test", "testte", "doğrula", "rapor", "dön", "donme", "return", "teslim", "birim test", "entegrasyon test", "oluşmasın", "201", "422", "açıklama", "aciklama", "yardım", "yardim", "çıkmalı", "cikmali", "paragraf"}
var constraintTerms = []string{"yalnızca", "sadece", "değiştirme", "koru", "bozma", "olmamalı", "kapsam", "must", "only", "without", "do not", "referans alma", "kullanma", "kullanmay", "önce", "sonra", "onay", "geri alma", "rollback", "durma", "duraklatma"}

// Evaluate is intentionally local: the same text always produces the same result.
func Evaluate(prompt string) Result {
	text := normalize(prompt)
	words := strings.Fields(text)
	kind := DetectKind(text)
	action := matches(text, actionTerms)
	context := matches(text, contextTerms) + concreteSignals(text)
	output := matches(text, outputTerms)
	constraints := matches(text, constraintTerms)
	vague := matches(text, vagueTerms) > 0 && (len(words) < 8 || context == 0)
	unknownContext := strings.Contains(text, "hangi veritabani") || strings.Contains(text, "which database")
	if unknownContext {
		context = 0
	}
	if kind == BugFix && (strings.Contains(text, "panic") || strings.Contains(text, "crash")) {
		action = max(action, 1)
	}
	if kind == Testing && concreteSignals(text) == 0 && len(words) < 5 {
		output = 0
	}
	if output == 1 && context > 1 && (strings.Contains(text, "test") || strings.Contains(text, "dogrula")) {
		output++
	}
	contradictory := contradiction(text)

	clarity := 15
	if action > 0 {
		clarity = 65
	}
	if len(words) >= 8 && !vague {
		clarity += 15
	}
	if context > 1 || output > 0 {
		clarity += 10
	}
	if len(words) < 5 {
		clarity -= 30
	}
	if vague {
		clarity -= 35
	}
	if contradictory {
		clarity -= 30
	}

	contextScore := evidenceScore(context)
	outputScore := evidenceScore(output)
	constraintScore := evidenceScore(constraints)
	if kind == APIContract && concreteSignals(text) == 0 {
		contextScore = min(contextScore, 35)
	}
	if kind == Content && output > 0 {
		constraintScore = max(constraintScore, 75)
	}
	if constraints > 0 {
		constraintScore = max(constraintScore, 75)
	}
	if needsVerification(kind) && output == 0 {
		outputScore = min(outputScore, 35)
	}
	if kind == ResearchPlan && constraints == 0 {
		constraintScore = min(constraintScore, 35)
	}
	if kind == ResearchPlan && !has(text, "markdown", "md dosyasi", "rapor", "teslim", "format", "cikti") {
		outputScore = min(outputScore, 25)
	}
	applicability := 100
	if action == 0 {
		applicability -= 30
	}
	if vague {
		applicability -= 25
	}
	if vague && context == 0 {
		applicability -= 20
	}
	if context == 0 {
		applicability -= 20
	}
	if len(words) < 5 {
		applicability -= 25
	}
	if output == 0 && needsVerification(kind) {
		applicability -= 20
	}
	if kind == ResearchPlan && !has(text, "markdown", "md dosyasi", "rapor", "teslim", "format", "cikti") {
		applicability = min(applicability, 55)
	}
	if contradictory {
		applicability -= 55
	}
	if contradictory {
		clarity, outputScore, constraintScore, applicability = min(clarity, 30), min(outputScore, 35), 15, 15
	}

	criteria := []Criterion{
		{Name: "Amaç ve Görev Netliği", Score: clamp(clarity)},
		{Name: "Bağlam ve Teknik Bilgi", Score: contextScore},
		{Name: "Beklenen Sonuç", Score: outputScore},
		{Name: "Kısıtlar ve Sınırlar", Score: constraintScore},
		{Name: "Belirsizlik / Uygulanabilirlik", Score: clamp(applicability)},
	}
	findings := findings(action, context, output, constraints, vague, contradictory, kind)
	return Result{
		Criteria: criteria, Findings: findings, Score: average(criteria), Kind: kind,
		NeedsContext: context == 0, NeedsFormat: output == 0 && needsVerification(kind),
		NeedsClarifying: action == 0 || vague || contradictory || len(words) < 5,
		Contradictory:   contradictory,
	}
}

func DetectKind(text string) TaskKind {
	switch {
	case has(text, "migration", "migrate", "rollback", "tablosu", "tablosundaki"):
		return DataMigration
	case has(text, "endpoint", "api", "http", "json olarak dön", "json olarak don"):
		return APIContract
	case has(text, "güvenlik", "guvenlik", "security", "parola", "auth"):
		return Security
	case has(text, "araştır", "arastir", "mimari", "aşamal", "asamali", "agile"):
		return ResearchPlan
	case has(text, "p95", "p99", "fps", "gecikme", "latency", "hızlandır", "hizlandir"):
		return Performance
	case has(text, "refactor", "tekrar eden", "sadeleştir", "sadelestir"):
		return Refactor
	case has(text, "test", "birim", "unit test"):
		return Testing
	case has(text, "readme", "dokümantasyon", "dokumantasyon"):
		return Documentation
	case has(text, "incele", "review", "koda bak"):
		return Review
	case has(text, "çevir", "cevir", "özetle", "ozetle"):
		return Content
	case has(text, "hata", "bug", "panic", "crash", "düzelt", "duzelt", "fix"):
		return BugFix
	case has(text, "ekle", "oluştur", "olustur", "add", "create"):
		return Feature
	default:
		return General
	}
}

func needsVerification(kind TaskKind) bool {
	return kind != Content
}

func normalize(value string) string {
	replacer := strings.NewReplacer("ç", "c", "Ç", "c", "ğ", "g", "Ğ", "g", "ı", "i", "I", "i", "İ", "i", "ö", "o", "Ö", "o", "ş", "s", "Ş", "s", "ü", "u", "Ü", "u")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
}

func has(text string, terms ...string) bool { return matches(text, terms) > 0 }

func matches(text string, terms []string) int {
	count := 0
	for _, term := range terms {
		term = normalize(term)
		if strings.ContainsAny(term, " ./`-") {
			if strings.Contains(text, term) {
				count++
			}
			continue
		}
		for _, word := range strings.Fields(text) {
			word = strings.Trim(word, ".,;:!?()[]{}\"'`")
			if word == term || strings.HasPrefix(word, term+"'") || (len(term) >= 4 && strings.HasPrefix(word, term)) {
				count++
				break
			}
		}
		if strings.Contains(text, term+":") {
			count++
		}
	}
	return count
}

func concreteSignals(text string) int {
	count := 0
	for _, word := range strings.Fields(text) {
		if strings.ContainsAny(word, "/._`-") || strings.IndexFunc(word, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
			count++
		}
	}
	return min(count, 3)
}

func contradiction(text string) bool {
	return (strings.Contains(text, "misafir") && strings.Contains(text, "oturum acmadan") && strings.Contains(text, "hicbir ayar degismesin")) ||
		(strings.Contains(text, "sadece") && strings.Contains(text, "yeni bir paket ekle"))
}

func evidenceScore(signals int) int {
	switch {
	case signals == 0:
		return 25
	case signals == 1:
		return 65
	case signals == 2:
		return 85
	default:
		return 95
	}
}

func findings(action, context, output, constraints int, vague, contradictory bool, kind TaskKind) []string {
	result := []string{}
	if contradictory {
		return []string{"Birbiriyle çelişen talimatlar var; önce bu çelişki çözülmeli."}
	}
	if action == 0 {
		result = append(result, "Yapılacak görev yeterince açık değil.")
	}
	if context == 0 {
		result = append(result, "Görevi uygulamak için gerekli bağlam eksik.")
	}
	if output == 0 && needsVerification(kind) {
		result = append(result, "Beklenen sonuç veya doğrulama ölçütü belirtilmemiş.")
	}
	if constraints == 0 && kind == ResearchPlan {
		result = append(result, "Araştırma kapsamı veya sınırları belirtilmemiş.")
	}
	if vague {
		result = append(result, "Belirsiz veya ölçülemeyen ifadeler var.")
	}
	return result
}

func clamp(score int) int { return max(0, min(100, score)) }
func average(criteria []Criterion) int {
	total := 0
	for _, criterion := range criteria {
		total += criterion.Score
	}
	return total / len(criteria)
}
