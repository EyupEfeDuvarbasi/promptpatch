// Package editor implements the terminal editor launched by Codex's Ctrl+G.
package editor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

var terminalInput = os.Stdin

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
	result := score.Evaluate(prompt)
	questions := cli.LocalQuestions(result)
	answers, complete := ask(questions)
	if !complete {
		return nil
	}
	clear()
	screenln("Prompt iyileştiriliyor…")
	screenln("Yerel model yanıtı hazırlanıyor.")
	client, err := llm.New(llm.Ollama, "")
	if err != nil {
		return rewriteFailed(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	assessment, err := client.Improve(ctx, prompt, questions, answers)
	if err != nil || !usableRewrite(prompt, assessment.ImprovedPrompt) {
		if err == nil {
			err = fmt.Errorf("yerel model güvenilir bir yeniden yazım üretmedi")
		}
		return rewriteFailed(err)
	}
	improvedPrompt := assessment.ImprovedPrompt
	improved := score.Evaluate(improvedPrompt)
	if !chooseComparison(prompt, result, improvedPrompt, improved) {
		return nil
	}
	return os.WriteFile(path, []byte(improvedPrompt+"\n"), 0600)
}

func usableRewrite(original, candidate string) bool {
	candidate = strings.ToLower(candidate)
	return len(strings.Fields(candidate)) >= 8 && normalizePrompt(original) != normalizePrompt(candidate) && !strings.Contains(candidate, "#") && !strings.Contains(candidate, "soru:") && !strings.Contains(candidate, "cevap:")
}

func normalizePrompt(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func rewriteFailed(err error) error {
	clear()
	screenln("İyileştirme güvenilir biçimde üretilemedi; özgün prompt korunuyor.")
	screenln(err)
	screenln("Enter veya Esc ile geri dön.")
	raw(func() bool { _ = readKey(); return true })
	return nil
}

func chooseComparison(original string, originalScore score.Result, improved string, improvedScore score.Result) bool {
	selected := 0
	return raw(func() bool {
		for {
			clear()
			printScoreSummary(originalScore, improvedScore)
			budget := promptLineBudget()
			printPrompt("Özgün prompt", original, budget)
			screenln()
			printPrompt("İyileştirilmiş prompt", improved, budget)
			screenln("\n↑/↓ ile seç, Enter ile onayla, Esc ile özgünü koru.")
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
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		tty = os.Stdin
	} else {
		defer tty.Close()
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
	second, ok := readByte()
	if !ok || (second != '[' && second != 'O') {
		return "esc"
	}
	third, ok := readByte()
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

func printScoreSummary(original, improved score.Result) {
	screenln("Puanlar: G görev · B bağlam · S sonuç · K kısıt · U uygulanabilirlik")
	screenln(scoreLine("Özgün", original))
	screenln(scoreLine("İyileştirilmiş", improved))
}

func scoreLine(label string, result score.Result) string {
	if len(result.Criteria) != 5 {
		return fmt.Sprintf("%s: %d/100", label, result.Score)
	}
	return fmt.Sprintf("%s: %d/100  G%d B%d S%d K%d U%d", label, result.Score, result.Criteria[0].Score, result.Criteria[1].Score, result.Criteria[2].Score, result.Criteria[3].Score, result.Criteria[4].Score)
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
	return max(3, (rows-13)/2)
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
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rc := filepath.Join(home, ".zshrc")
	if strings.HasSuffix(os.Getenv("SHELL"), "bash") {
		rc = filepath.Join(home, ".bashrc")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	editorPath := filepath.Join(home, ".local", "share", "promptpatch", "bin", "promptpatch-codex-editor")
	if err := os.MkdirAll(filepath.Dir(editorPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(editorPath, []byte(wrapperScript(executable)), 0700); err != nil {
		return err
	}
	contents, _ := os.ReadFile(rc)
	block := codexBlock(editorPath)
	updated := replaceCodexBlock(string(contents), block)
	if updated == string(contents) {
		return nil
	}
	return os.WriteFile(rc, []byte(updated), 0600)
}

const codexMarker = "# promptcheck Codex editor"

func codexBlock(editorPath string) string {
	editor := shellQuote(editorPath)
	return "\n" + codexMarker + "\ncodex() { VISUAL=" + editor + " EDITOR=" + editor + " command codex \"$@\"; }\n"
}

func wrapperScript(executable string) string {
	return "#!/bin/sh\nexec " + shellQuote(executable) + " edit \"$@\"\n"
}

func replaceCodexBlock(contents, block string) string {
	start := strings.Index(contents, codexMarker)
	if start < 0 {
		return contents + block
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
}
