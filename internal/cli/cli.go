// Package cli implements promptcheck's terminal command interface.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

func Run(ctx context.Context, args []string, in io.Reader, out io.Writer, client *llm.Client) error {
	_, _, _ = ctx, in, client
	if wantsHelp(args) {
		fmt.Fprint(out, HelpText())
		return nil
	}
	return errors.New("doğrudan CLI prompt girişi kaldırıldı; Codex içinde Ctrl-G kullanın")
}

func wantsHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help")
}

func HelpText() string {
	return `promptcheck - AI kodlama promptlarını puanla ve iyileştir

Kullanım:
  promptcheck setup-codex
  promptcheck configure-context
  promptcheck serve
  promptcheck edit <dosya>

Komutlar:
  setup-codex   Codex EDITOR/VISUAL entegrasyonunu kurar ve yakın bağlam ayarını sorar.
  configure-context  Yakın sohbet bağlamı ayarını yeniden seçtirir.
  serve         HTTP API sunucusunu başlatır; server tarafındaki private Ollama'yı kullanır.
  edit <dosya>  EDITOR/VISUAL akışından çağrılır; dosyadaki taslak promptu iyileştirir.

Doğrudan promptcheck "<prompt>" kullanımı kaldırıldı. Prompt geliştirme akışı
Codex içinde Ctrl-G ile çalışır.

Server modu için temel değişkenler: PROMPTPATCH_SERVER_ADDR, PROMPTPATCH_SERVER_TOKEN,
PROMPTPATCH_OLLAMA_URL, PROMPTPATCH_OLLAMA_MODEL, PROMPTPATCH_MAX_CONCURRENCY.
`
}

func LocalQuestions(result score.Result) []string {
	return LocalQuestionsWithContext(result, "", "")
}

// LocalQuestionsWithContext asks only for information absent from the draft
// and the explicitly supplied conversation context.
func LocalQuestionsWithContext(result score.Result, prompt, chatContext string) []string {
	questions := []string{}
	contextText := strings.ToLower(strings.TrimSpace(chatContext + "\n" + prompt))
	if result.NeedsContext && !contextHasTaskContext(contextText) {
		questions = append(questions, contextQuestion(result.Kind))
	}
	if result.NeedsFormat && !contextHasExpectedOutput(contextText) {
		questions = append(questions, outputQuestion(result.Kind))
	}
	if result.NeedsClarifying && !contextHasTaskContext(contextText) {
		questions = append(questions, "Tam olarak neyin değişmesini istiyorsunuz?")
	}
	if len(questions) > 2 {
		return questions[:2]
	}
	return questions
}

func contextHasTaskContext(contextText string) bool {
	if len(strings.Fields(contextText)) < 5 {
		return false
	}
	hasReference := containsAny(contextText,
		".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java",
		"dosya", "fonksiyon", "function", "endpoint", "api", "bileşen", "component",
		"sayfa", "ekran", "terminal", "cli", "tablo", "veritaban")
	hasTask := containsAny(contextText,
		"düzelt", "duzelt", "fix", "ekle", "add", "oluştur", "olustur", "güncelle", "guncelle",
		"refactor", "incele", "araştır", "arastir", "test", "migration", "çevir", "cevir")
	return hasReference && hasTask
}

func contextHasExpectedOutput(contextText string) bool {
	if len(strings.Fields(contextText)) < 5 {
		return false
	}
	return containsAny(contextText,
		"beklenen", "kabul kriter", "beklenen davranış", "beklenen sonuc", "çıktı", "cikti",
		"sonuç", "sonuc", "format", "json", "markdown", "rapor", "return", "dön", "don", "test")
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func contextQuestion(kind score.TaskKind) string {
	switch kind {
	case score.BugFix:
		return "Hangi dosya veya bileşende, hangi hata belirtisi görülüyor?"
	case score.APIContract:
		return "Hangi endpoint, yöntem veya veri sözleşmesi değişecek?"
	case score.Performance:
		return "Hangi iş yükü için mevcut ölçüm ve hedef değer nedir?"
	case score.DataMigration:
		return "Hangi veri kaynağı ve hangi geçiş kapsamı söz konusu?"
	case score.Security:
		return "Hangi akış veya varlık için güvenlik riski ele alınacak?"
	default:
		return "Hangi dosya, bileşen veya teknolojiyle ilgili?"
	}
}

func outputQuestion(kind score.TaskKind) string {
	switch kind {
	case score.Testing:
		return "Hangi davranış testlerle doğrulanmalı?"
	case score.ResearchPlan:
		return "Plan hangi formatta ve hangi kararları içerecek?"
	default:
		return "Beklenen davranış veya çıktı formatı nedir?"
	}
}

func LocalImprove(prompt string, questions, answers []string) string {
	return LocalImproveWithContext(prompt, "", questions, answers)
}

// LocalImproveWithContext preserves the latest user context when no model backend is available.
func LocalImproveWithContext(prompt, chatContext string, questions, answers []string) string {
	task, criteria := splitAcceptanceCriteria(prompt)
	sections := []string{"# " + promptKind(task), "\n## Amaç\n" + strings.TrimSpace(task)}
	if context := lastUserContext(chatContext); context != "" {
		sections = append(sections, "\n## Bağlam\n- "+strings.ReplaceAll(context, "\n", "\n- "))
	}
	for i, answer := range answers {
		answer = strings.TrimSpace(answer)
		if answer == "" || i >= len(questions) {
			continue
		}
		sections = append(sections, "\n## "+localSection(questions[i])+"\n- "+answer)
	}
	if criteria != "" {
		sections = append(sections, "\n## Kabul kriterleri\n"+criteria)
	}
	return strings.TrimSpace(strings.Join(sections, "\n"))
}

func lastUserContext(value string) string {
	blocks := strings.Split(strings.TrimSpace(value), "\n\n")
	for index := len(blocks) - 1; index >= 0; index-- {
		block := strings.TrimSpace(blocks[index])
		if len(block) >= len("USER:") && strings.EqualFold(block[:len("USER:")], "USER:") {
			return strings.TrimSpace(block[len("USER:"):])
		}
	}
	return ""
}

func localSection(question string) string {
	question = strings.ToLower(question)
	if strings.Contains(question, "dosya") || strings.Contains(question, "fonksiyon") || strings.Contains(question, "teknoloji") {
		return "Bağlam"
	}
	if strings.Contains(question, "davranış") || strings.Contains(question, "sonuç") || strings.Contains(question, "çıktı") || strings.Contains(question, "format") {
		return "Beklenen sonuç"
	}
	return "Yapılacak değişiklik"
}

func promptKind(prompt string) string {
	text := strings.ToLower(prompt)
	switch {
	case strings.Contains(text, "düzelt") || strings.Contains(text, "hata") || strings.Contains(text, "bug") || strings.Contains(text, "fix"):
		return "Hata düzeltme"
	case strings.Contains(text, "refactor") || strings.Contains(text, "yeniden düzenle"):
		return "Refactor"
	case strings.Contains(text, "ekle") || strings.Contains(text, "oluştur") || strings.Contains(text, "add") || strings.Contains(text, "create"):
		return "Özellik geliştirme"
	default:
		return "Geliştirme görevi"
	}
}

func splitAcceptanceCriteria(prompt string) (string, string) {
	text := strings.TrimSpace(prompt)
	lower := strings.ToLower(text)
	for _, marker := range []string{"kabul kriterleri:", "acceptance criteria:"} {
		if index := strings.Index(lower, marker); index >= 0 {
			return strings.TrimSpace(text[:index]), strings.TrimSpace(text[index+len(marker):])
		}
	}
	return text, ""
}

func readPrompt(args []string, in io.Reader) (string, error) {
	if len(args) > 1 {
		return "", errors.New("yalnızca bir prompt verin")
	}
	if len(args) == 1 {
		return args[0], nil
	}
	text, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(text)) == "" {
		return "", errors.New("prompt gerekli")
	}
	return strings.TrimSpace(string(text)), nil
}

func ask(out io.Writer, in io.Reader, questions []string) (string, error) {
	reader := bufio.NewReader(in)
	answers := make([]string, len(questions))
	for i, question := range questions {
		fmt.Fprintf(out, "%s\n> ", question)
		answer, err := reader.ReadString('\n')
		if err != nil && len(answer) == 0 {
			return "", errors.New("sorular için etkileşimli terminal gerekli")
		}
		answers[i] = strings.TrimSpace(answer)
	}
	return strings.Join(answers, "\n"), nil
}

func blend(rules score.Result, semantic []score.Criterion) score.Result {
	semanticByName := make(map[string]score.Criterion, len(semantic))
	for _, criterion := range semantic {
		semanticByName[criterion.Name] = criterion
	}
	criteria := make([]score.Criterion, len(rules.Criteria))
	total := 0
	for i, rule := range rules.Criteria {
		value := rule.Score
		if criterion, ok := semanticByName[rule.Name]; ok {
			value = (rule.Score*40 + criterion.Score*60) / 100
		}
		criteria[i] = score.Criterion{Name: rule.Name, Score: value}
		total += value
	}
	return score.Result{Criteria: criteria, Score: total / len(criteria)}
}

func printDetail(out io.Writer, title, prompt string, result score.Result) {
	fmt.Fprintf(out, "%s — %d/100\n%s\n", title, result.Score, prompt)
	for _, criterion := range result.Criteria {
		fmt.Fprintf(out, "  %s: %d/100\n", criterion.Name, criterion.Score)
	}
}

func summary(findings []string) string {
	if len(findings) == 0 {
		return "İstek yeterince somut görünüyor."
	}
	if len(findings) > 2 {
		findings = findings[:2]
	}
	return strings.Join(findings, " ")
}

func copyToClipboard(text string) error {
	commands := [][]string{{"pbcopy"}}
	switch runtime.GOOS {
	case "linux":
		commands = [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
	case "windows":
		commands = [][]string{{"clip.exe"}}
	}
	for _, command := range commands {
		path, err := exec.LookPath(command[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, command[1:]...)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return fmt.Errorf("pano aracı bulunamadı: %s kurun", clipboardTools(commands))
}

func clipboardTools(commands [][]string) string {
	names := make([]string, len(commands))
	for i, command := range commands {
		names[i] = command[0]
	}
	return strings.Join(names, ", ")
}
