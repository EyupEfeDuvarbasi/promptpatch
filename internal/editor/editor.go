// Package editor implements the terminal editor launched by Codex's Ctrl+G.
package editor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

func Run(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	prompt := strings.TrimSpace(string(contents))
	if prompt == "" {
		return nil
	}
	result := score.Evaluate(prompt)
	if !choose(prompt, result, "İyileştir", "Özgün promptu koru") {
		return nil
	}
	questions := cli.LocalQuestions(result)
	answers := ask(questions)
	improved := cli.LocalImprove(prompt, questions, answers)
	if !chooseComparison(prompt, result, improved, score.Evaluate(improved)) {
		return nil
	}
	return os.WriteFile(path, []byte(improved+"\n"), 0600)
}

func choose(prompt string, result score.Result, actions ...string) bool {
	selected := 0
	return raw(func() bool {
		for {
			clear()
			printPrompt("Prompt", prompt)
			fmt.Printf("\nPuan: %d/100\n", result.Score)
			for _, criterion := range result.Criteria {
				fmt.Printf("  %-26s %d/100\n", criterion.Name+":", criterion.Score)
			}
			fmt.Println("\n↑/↓ ile seç, Enter ile onayla, Esc ile çık.")
			printActions(actions, selected)
			switch readKey() {
			case "up":
				selected = (selected + len(actions) - 1) % len(actions)
			case "down":
				selected = (selected + 1) % len(actions)
			case "enter":
				return selected == 0
			case "esc":
				return false
			}
		}
	})
}

func chooseComparison(original string, originalScore score.Result, improved string, improvedScore score.Result) bool {
	selected := 0
	return raw(func() bool {
		for {
			clear()
			printPrompt(fmt.Sprintf("Özgün prompt — %d/100", originalScore.Score), original)
			fmt.Println()
			printPrompt(fmt.Sprintf("İyileştirilmiş prompt — %d/100", improvedScore.Score), improved)
			fmt.Println("\n↑/↓ ile seç, Enter ile onayla, Esc ile özgünü koru.")
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

func ask(questions []string) []string {
	if len(questions) == 0 {
		return nil
	}
	fmt.Print("\033[2J\033[H")
	reader := bufio.NewReader(os.Stdin)
	answers := make([]string, len(questions))
	for i, question := range questions {
		fmt.Printf("%s\n> ", question)
		answer, _ := reader.ReadString('\n')
		answers[i] = strings.TrimSpace(answer)
	}
	return answers
}

func raw(run func() bool) bool {
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return false
	}
	defer term.Restore(int(os.Stdin.Fd()), state)
	return run()
}

func readKey() string {
	var input [3]byte
	if n, _ := os.Stdin.Read(input[:1]); n == 0 {
		return "esc"
	}
	if input[0] == '\r' || input[0] == '\n' {
		return "enter"
	}
	if input[0] != 27 {
		return ""
	}
	if n, _ := os.Stdin.Read(input[1:3]); n != 2 {
		return "esc"
	}
	if input[2] == 'A' {
		return "up"
	}
	if input[2] == 'B' {
		return "down"
	}
	return "esc"
}

func clear() { fmt.Print("\033[2J\033[H") }

func printActions(actions []string, selected int) {
	for i, action := range actions {
		prefix := "  "
		if i == selected {
			prefix = "❯ "
		}
		fmt.Println(prefix + action)
	}
}

func printPrompt(title, value string) {
	fmt.Println(title + ":")
	for _, line := range wrap(value, width()-2) {
		fmt.Println("  " + line)
	}
}

func width() int {
	columns, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || columns < 30 {
		return 80
	}
	return columns
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
	const marker = "# promptcheck Codex editor"
	contents, _ := os.ReadFile(rc)
	if strings.Contains(string(contents), marker) {
		return nil
	}
	block := "\n" + marker + "\ncodex() { VISUAL='promptcheck edit' EDITOR='promptcheck edit' command codex \"$@\"; }\n"
	file, err := os.OpenFile(rc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(block)
	return err
}
