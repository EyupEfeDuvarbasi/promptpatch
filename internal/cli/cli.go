// Package cli implements promptcheck's terminal command interface.
package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

func Run(ctx context.Context, args []string, in io.Reader, out io.Writer, client *llm.Client) error {
	flags := flag.NewFlagSet("promptcheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	detail := flags.Bool("detail", false, "")
	flags.BoolVar(detail, "d", false, "")
	copyResult := flags.Bool("copy", false, "")
	flags.BoolVar(copyResult, "c", false, "")
	model := flags.String("model", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	prompt, err := readPrompt(flags.Args(), in)
	if err != nil {
		return err
	}
	if *model != "" {
		if client == nil {
			return errors.New("--model için API anahtarı yapılandırması gerekli")
		}
		client.Model = *model
	}

	originalRules := score.Evaluate(prompt)
	improvedPrompt := LocalImprove(prompt, nil, nil)
	original := originalRules
	improved := score.Evaluate(improvedPrompt)
	asked := false
	if client != nil {
		originalLLM, err := client.Assess(ctx, prompt)
		if err != nil {
			return err
		}
		improvedLLM := originalLLM
		if len(originalLLM.Questions) > 0 {
			answers, err := ask(out, in, originalLLM.Questions)
			if err != nil {
				return err
			}
			asked = true
			improvedLLM, err = client.Assess(ctx, prompt+"\n\nEk bilgiler:\n"+answers+"\nBu bilgilerle iyileştirilmiş promptu üret; soru sorma.")
			if err != nil {
				return err
			}
			if len(improvedLLM.Questions) > 0 || improvedLLM.ImprovedPrompt == "" {
				return errors.New("ek bilgilerden sonra iyileştirilmiş prompt üretilemedi")
			}
		}
		if improvedLLM.ImprovedPrompt == "" {
			return errors.New("iyileştirilmiş prompt üretilemedi")
		}
		improvedPrompt = improvedLLM.ImprovedPrompt
		original = blend(originalRules, originalLLM.Criteria)
		improved = blend(score.Evaluate(improvedPrompt), improvedLLM.ImprovedCriteria)
	} else if questions := LocalQuestions(originalRules); len(questions) > 0 {
		answers, err := ask(out, in, questions)
		if err != nil {
			return err
		}
		asked = true
		improvedPrompt = LocalImprove(prompt, questions, strings.Split(answers, "\n"))
		improved = score.Evaluate(improvedPrompt)
	}
	if *detail {
		printDetail(out, "Özgün Prompt", prompt, original)
		printDetail(out, "İyileştirilmiş Prompt", improvedPrompt, improved)
	} else {
		fmt.Fprintf(out, "Puan: %d/100 — %s\n", original.Score, summary(originalRules.Findings))
		fmt.Fprintf(out, "İyileştirilmiş Puan: %d/100\n", improved.Score)
		if asked {
			fmt.Fprintf(out, "Özgün Prompt:\n%s\nİyileştirilmiş Prompt:\n%s\n", prompt, improvedPrompt)
		}
	}
	if *copyResult {
		if err := copyToClipboard(improvedPrompt); err != nil {
			return err
		}
		fmt.Fprintln(out, "İyileştirilmiş prompt panoya kopyalandı.")
	}
	return nil
}

func LocalQuestions(result score.Result) []string {
	questions := []string{}
	if result.NeedsContext {
		questions = append(questions, "Hangi dosya, fonksiyon veya teknolojiyle ilgili?")
	}
	if result.NeedsFormat {
		questions = append(questions, "Beklenen davranış veya çıktı formatı nedir?")
	}
	if result.NeedsClarifying && len(questions) < 2 {
		questions = append(questions, "Tam olarak neyin değişmesini istiyorsunuz?")
	}
	if len(questions) > 2 {
		return questions[:2]
	}
	return questions
}

func LocalImprove(prompt string, questions, answers []string) string {
	result := "Görev:\n" + strings.TrimSpace(prompt)
	for i, answer := range answers {
		answer = strings.TrimSpace(answer)
		if answer == "" || i >= len(questions) {
			continue
		}
		if !strings.Contains(result, "\nEk bağlam:\n") {
			result += "\n\nEk bağlam:\n"
		}
		result += "- " + questions[i] + " " + answer + "\n"
	}
	return strings.TrimSpace(result)
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
	criteria := make([]score.Criterion, len(rules.Criteria))
	total := 0
	for i, rule := range rules.Criteria {
		value := rule.Score
		if i < len(semantic) {
			value = (rule.Score*40 + semantic[i].Score*60) / 100
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
	if runtime.GOOS == "linux" {
		commands = [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
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
	return errors.New("pano aracı bulunamadı: pbcopy, wl-copy, xclip veya xsel kurun")
}
