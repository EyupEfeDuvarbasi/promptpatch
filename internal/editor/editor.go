// Package editor implements the terminal editor launched by Codex's Ctrl+G.
package editor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/api"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/chat"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/config"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

var (
	terminalInput = os.Stdin
	getenv        = os.Getenv
)

const minimumMascotDuration = 250 * time.Millisecond

func Run(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	prompt := strings.TrimSpace(string(contents))
	if prompt == "" {
		return nil
	}
	enterAlternateScreen()
	defer leaveAlternateScreen()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	chatContext := nearbyContext()
	var improvement editorImprovement
	var complete bool
	var questions, answers []string
	for {
		improvement, questions, complete = improveWithMascot(ctx, prompt, chatContext.Text, questions, answers)
		if !complete {
			return nil
		}
		if len(questions) == 0 {
			break
		}
		answers, complete = ask(questions)
		if !complete {
			return nil
		}
	}
	if !complete {
		return nil
	}
	contextSource := comparisonSource(chatContext.Source, improvement.Source)
	if !showableImprovement(improvement) {
		return rewriteFailed(fmt.Errorf("üretilen prompt güvenilir değil: %s -> %s", scoreBadge(improvement.Original.Score), scoreBadge(improvement.Improved.Score)))
	}
	if !chooseComparison(prompt, improvement.Original, improvement.Prompt, improvement.Improved, contextSource) {
		return nil
	}
	return os.WriteFile(path, []byte(improvement.Prompt+"\n"), 0600)
}

type editorImprovement struct {
	Prompt   string
	Source   string
	Original score.Result
	Improved score.Result
}

func improveWithMascot(ctx context.Context, prompt, chatContext string, questions, answers []string) (editorImprovement, []string, bool) {
	clear()
	screenln("Prompt iyileştiriliyor…")
	screenln("Model yanıtı hazırlanıyor.")
	hideCursor()
	started := time.Now()
	stopMascot := startMascot(os.Stdout, width, colorEnabled())
	improvement, nextQuestions, complete := improveWithBestAvailable(ctx, prompt, chatContext, questions, answers)
	if remaining := minimumMascotDuration - time.Since(started); remaining > 0 {
		timer := time.NewTimer(remaining)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
		}
	}
	stopMascot()
	showCursor()
	clear()
	return improvement, nextQuestions, complete
}

func improveWithBestAvailable(ctx context.Context, prompt, chatContext string, questions, answers []string) (editorImprovement, []string, bool) {
	// Ask deterministic, decision-changing questions before invoking a backend.
	// This keeps the UX consistent when no remote server is configured.
	if len(questions) == 0 && len(answers) == 0 {
		if nextQuestions := cli.LocalQuestionsWithContext(score.Evaluate(prompt), prompt, chatContext); len(nextQuestions) > 0 {
			return editorImprovement{}, nextQuestions, true
		}
	}
	if response, ok := improveWithRemoteServer(ctx, prompt, chatContext, questions, answers); ok {
		if len(response.Questions) > 0 && strings.TrimSpace(response.ImprovedPrompt) == "" {
			return editorImprovement{}, response.Questions, true
		} else if strings.TrimSpace(response.ImprovedPrompt) != "" {
			return improvementFromAPI(response), nil, true
		}
	}
	if improved, ok := improveWithLocalOllama(ctx, prompt, chatContext, questions, answers); ok {
		original := score.Evaluate(prompt)
		improvedScore := score.Evaluate(improved)
		return editorImprovement{Prompt: improved, Source: "ollama", Original: original, Improved: improvedScore}, nil, true
	}
	improved := cli.LocalImproveWithContext(prompt, chatContext, questions, answers)
	return editorImprovement{Prompt: improved, Source: "Yerel fallback", Original: score.Evaluate(prompt), Improved: score.Evaluate(improved)}, nil, true
}

func improveWithRemoteServer(ctx context.Context, prompt, chatContext string, questions, answers []string) (api.ImproveResponse, bool) {
	path, err := config.DefaultPath()
	if err != nil {
		return api.ImproveResponse{}, false
	}
	url, token, ok := config.RemoteServer(path)
	if !ok {
		return api.ImproveResponse{}, false
	}
	if !config.RemoteContextEnabled(path) {
		chatContext = ""
	}
	response, err := api.Client{URL: url, Token: token}.Improve(ctx, api.ImproveRequest{
		Prompt: prompt, Questions: questions, Answers: answers, ChatContext: chatContext,
	})
	if err != nil {
		return api.ImproveResponse{}, false
	}
	if strings.TrimSpace(response.ImprovedPrompt) == "" {
		return response, len(response.Questions) > 0
	}
	if (response.ImprovedScore != 0 || response.OriginalScore != 0) && response.ImprovedScore < response.OriginalScore {
		return api.ImproveResponse{}, false
	}
	if !cli.ValidRewrite(prompt, response.ImprovedPrompt) {
		return api.ImproveResponse{}, false
	}
	return response, true
}

func improveWithLocalOllama(ctx context.Context, prompt, chatContext string, questions, answers []string) (string, bool) {
	client, err := llm.New(llm.Ollama, "")
	if err != nil {
		return "", false
	}
	assessment, err := client.ImproveWithContext(ctx, prompt, chatContext, questions, answers)
	if err != nil || !cli.ValidRewrite(prompt, assessment.ImprovedPrompt) {
		return "", false
	}
	improved := score.Evaluate(assessment.ImprovedPrompt)
	original := score.Evaluate(prompt)
	if improved.Score < original.Score {
		return "", false
	}
	return assessment.ImprovedPrompt, true
}

func improvementFromAPI(response api.ImproveResponse) editorImprovement {
	return editorImprovement{
		Prompt: response.ImprovedPrompt, Source: sourceLabel(response.Source),
		Original: resultFromAPI(response.OriginalScore, response.Original),
		Improved: resultFromAPI(response.ImprovedScore, response.Improved),
	}
}

func showableImprovement(improvement editorImprovement) bool {
	return improvement.Improved.Score >= improvement.Original.Score && strings.TrimSpace(improvement.Prompt) != ""
}

func sourceLabel(source string) string {
	if source == "local" {
		return "Yerel fallback"
	}
	return "Server/" + source
}

func usableRewrite(original, candidate string) bool { return cli.ValidRewrite(original, candidate) }

func resultFromAPI(total int, criteria []score.Criterion) score.Result {
	if total == 0 && len(criteria) > 0 {
		for _, criterion := range criteria {
			total += criterion.Score
		}
		total /= len(criteria)
	}
	return score.Result{Score: total, Criteria: criteria}
}

func comparisonSource(contextSource, rewriteSource string) string {
	if rewriteSource == "" {
		return contextSource
	}
	source := "İyileştirme kaynağı: " + rewriteSource
	if contextSource == "" {
		return source
	}
	return contextSource + " · " + source
}

func rewriteFailed(err error) error {
	clear()
	screenln("İyileştirme güvenilir biçimde üretilemedi; özgün prompt korunuyor.")
	screenln(err)
	screenln("Enter veya Esc ile geri dön.")
	raw(func() bool { _ = readKey(); return true })
	return nil
}

func chooseComparison(original string, originalScore score.Result, improved string, improvedScore score.Result, contextSource string) bool {
	selected := 0
	return raw(func() bool {
		for {
			clear()
			printComparisonHeader(originalScore, improvedScore, contextSource)
			budget := promptLineBudget()
			printPrompt("Özgün prompt  ·  "+scoreBadge(originalScore.Score), original, budget)
			screenln()
			printPrompt("İyileştirilmiş prompt  ·  "+scoreBadge(improvedScore.Score), improved, budget)
			screenln("\n↑/↓ seç  ·  Enter uygula  ·  Esc özgünü koru")
			printActions([]string{"İyileştirilmiş promptu uygula", "Özgün promptu koru"}, selected)
			switch readKey() {
			case "up", "down":
				selected = 1 - selected
			case "enter":
				return selected == 0
			case "esc":
				return false
			}
		}
	})
}

func ask(questions []string) ([]string, bool) {
	if len(questions) == 0 {
		return nil, true
	}
	answers := make([]string, len(questions))
	complete := raw(func() bool {
		for i, question := range questions {
			clear()
			screenln("Eksik bilgi", i+1, "/", len(questions))
			screenln(question)
			screenln("Cevabını yaz ve Enter'a bas. Backspace ile silebilirsin.")
			screenf("> ")
			var answered bool
			answers[i], answered = readAnswer()
			if !answered {
				return false
			}
		}
		return true
	})
	return answers, complete
}

func readAnswer() (string, bool) {
	var answer []byte
	for {
		key, ok := readByte()
		if !ok || key == 3 { // Ctrl+C
			return "", false
		}
		switch key {
		case '\r', '\n':
			return strings.TrimSpace(string(answer)), true
		case 8, 127:
			answer = removeLastRune(answer)
			fmt.Print("\r\033[2K")
			screenf("> %s", string(answer))
		default:
			if key >= 32 {
				answer = append(answer, key)
				screenByte(key)
			}
		}
	}
}

func removeLastRune(value []byte) []byte {
	if len(value) == 0 {
		return value
	}
	_, size := utf8.DecodeLastRune(value)
	return value[:len(value)-size]
}

func raw(run func() bool) bool {
	tty, closeTTY, err := openTTY()
	if err != nil {
		tty = os.Stdin
	} else {
		defer closeTTY()
	}
	previousInput := terminalInput
	terminalInput = tty
	defer func() { terminalInput = previousInput }()
	state, err := term.MakeRaw(int(tty.Fd()))
	if err != nil {
		return false
	}
	defer term.Restore(int(tty.Fd()), state)
	return run()
}

func readKey() string {
	first, ok := readByte()
	if !ok {
		return "esc"
	}
	if first == '\r' || first == '\n' {
		return "enter"
	}
	if first != 27 {
		return ""
	}
	second, ok := readByteWithin(60 * time.Millisecond)
	if !ok || (second != '[' && second != 'O') {
		return "esc"
	}
	third, ok := readByteWithin(60 * time.Millisecond)
	if !ok {
		return "esc"
	}
	if third == 'A' {
		return "up"
	}
	if third == 'B' {
		return "down"
	}
	return "esc"
}

func readByte() (byte, bool) {
	var input [1]byte
	n, err := terminalInput.Read(input[:])
	return input[0], err == nil && n == 1
}

func clear() { fmt.Print("\033[2J\033[H") }

func enterAlternateScreen() { fmt.Print("\033[?1049h\033[2J\033[H") }

func leaveAlternateScreen() { fmt.Print("\033[?1049l") }

// screenf preserves line starts while the terminal is in raw input mode.
func screenf(format string, values ...any) {
	fmt.Print(screenText(fmt.Sprintf(format, values...)))
}

func screenln(values ...any) { screenf("%s\n", fmt.Sprint(values...)) }

func screenText(value string) string { return strings.ReplaceAll(value, "\n", "\r\n") }

func screenByte(value byte) { _, _ = os.Stdout.Write([]byte{value}) }

func printActions(actions []string, selected int) {
	for i, action := range actions {
		prefix := "  "
		if i == selected {
			prefix = "❯ "
		}
		screenln(prefix + action)
	}
}

func printPrompt(title, value string, limit int) {
	screenln(title + ":")
	for _, line := range promptPreview(value, width()-2, limit) {
		screenln("  " + line)
	}
}

func promptPreview(value string, columns, limit int) []string {
	lines := wrap(value, columns)
	if len(lines) <= limit {
		return lines
	}
	remaining := len(lines) - limit + 1
	return append(lines[:limit-1], fmt.Sprintf("… (%d satır daha)", remaining))
}

func printComparisonHeader(original, improved score.Result, contextSource string) {
	delta := improved.Score - original.Score
	screenln("PromptPatch")
	screenln("Özgün " + scoreBadge(original.Score) + "   →   İyileştirilmiş " + scoreBadge(improved.Score) + scoreDelta(delta))
	screenln("Puan: görev, bağlam, çıktı, kısıt ve uygulanabilirliğin ortalaması")
	if contextSource != "" {
		screenln("Yakın sohbet bağlamı: " + contextSource)
	}
	screenln(strings.Repeat("─", min(width(), 72)))
}

func nearbyContext() chat.Result {
	path, err := config.DefaultPath()
	if err != nil {
		return chat.Result{}
	}
	settings, err := config.Load(path)
	if err != nil || !settings.ChatContextSet || settings.ChatContextWords == 0 {
		return chat.Result{}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return chat.Result{}
	}
	return chat.Load(cwd, os.Getenv("PROMPTPATCH_HOST"), settings.ChatContextWords)
}

func scoreBadge(score int) string { return fmt.Sprintf("%d/100", score) }

func scoreDelta(delta int) string {
	if delta > 0 {
		return fmt.Sprintf("  (+%d)", delta)
	}
	if delta < 0 {
		return fmt.Sprintf("  (%d)", delta)
	}
	return "  (aynı)"
}

func width() int {
	columns, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || columns < 30 {
		return 80
	}
	return columns
}

func promptLineBudget() int {
	_, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || rows < 18 {
		rows = 24
	}
	return max(3, (rows-14)/2)
}

func wrap(value string, columns int) []string {
	var lines []string
	for _, paragraph := range strings.Split(value, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if len([]rune(word)) > columns {
				if line != "" {
					lines, line = append(lines, line), ""
				}
				letters := []rune(word)
				for len(letters) > columns {
					lines = append(lines, string(letters[:columns]))
					letters = letters[columns:]
				}
				line = string(letters)
			} else if line != "" && len([]rune(line))+1+len([]rune(word)) > columns {
				lines, line = append(lines, line), word
			} else if line == "" {
				line = word
			} else {
				line += " " + word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func SetupCodex() error {
	return setupCodex()
}

const (
	codexMarker    = "# promptcheck Codex editor"
	codexEndMarker = "# end promptcheck Codex editor"
)

func replaceCodexBlock(contents, block string) string {
	start := strings.Index(contents, codexMarker)
	if start < 0 {
		return contents + block
	}
	if end := strings.Index(contents[start:], codexEndMarker); end >= 0 {
		end += start + len(codexEndMarker)
		if next := strings.Index(contents[end:], "\n"); next >= 0 {
			end += next + 1
		}
		return contents[:start] + block + contents[end:]
	}
	end := strings.Index(contents[start:], "\n")
	if end < 0 {
		return contents[:start] + strings.TrimPrefix(block, "\n")
	}
	end += start + 1
	endLine := strings.Index(contents[end:], "\n")
	if endLine < 0 {
		return contents[:start] + block
	}
	end += endLine + 1
	return contents[:start] + block + contents[end:]
}
