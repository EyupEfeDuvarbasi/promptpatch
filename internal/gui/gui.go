// Package gui provides promptcheck's local desktop window.
package gui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

func Run() {
	a := app.NewWithID("com.promptpatch.app")
	w := a.NewWindow("Promptcheck")
	w.Resize(fyne.NewSize(620, 480))
	prompt := widget.NewMultiLineEntry()
	prompt.SetPlaceHolder("Promptunuzu yazın…")
	result := widget.NewMultiLineEntry()
	result.Disable()
	body := container.NewBorder(widget.NewLabel("Prompt"), nil, nil, nil, prompt)

	show := func(text string) {
		result.SetText(text)
		w.SetContent(container.NewBorder(nil, nil, nil, nil, container.NewVSplit(body, result)))
	}
	var evaluate func()
	evaluate = func() {
		text := strings.TrimSpace(prompt.Text)
		if text == "" {
			show("Prompt gerekli.")
			return
		}
		rules := score.Evaluate(text)
		questions := cli.LocalQuestions(rules)
		if len(questions) == 0 {
			show(format(text, rules, cli.LocalImprove(text)))
			return
		}
		answers := make([]*widget.Entry, len(questions))
		items := []fyne.CanvasObject{widget.NewLabel("Kısa sorular")}
		for i, question := range questions {
			items = append(items, widget.NewLabel(question))
			answers[i] = widget.NewEntry()
			items = append(items, answers[i])
		}
		items = append(items, widget.NewButton("İyileştir", func() {
			values := make([]string, len(answers))
			for i, answer := range answers {
				values[i] = strings.TrimSpace(answer.Text)
			}
			improved := cli.LocalImprove(text + "\n\nEk bilgiler:\n" + strings.Join(values, "\n"))
			show(format(text, rules, improved))
		}))
		w.SetContent(container.NewVScroll(container.NewVBox(items...)))
	}
	w.SetContent(container.NewBorder(nil, widget.NewButton("Değerlendir", evaluate), nil, nil, body))
	w.ShowAndRun()
}

func format(original string, originalScore score.Result, improved string) string {
	improvedScore := score.Evaluate(improved)
	return "Özgün Prompt — " + strconv.Itoa(originalScore.Score) + "/100\n" + original + "\n\nİyileştirilmiş Prompt — " + strconv.Itoa(improvedScore.Score) + "/100\n" + improved
}
