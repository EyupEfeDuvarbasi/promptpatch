package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

type corpusCase struct {
	ID            string   `json:"id"`
	Prompt        string   `json:"prompt"`
	ExpectedScore [2]int   `json:"expected_score"`
	Questions     []string `json:"questions"`
	MustKeep      []string `json:"must_keep"`
	MustNotInvent []string `json:"must_not_invent"`
}

func main() {
	file, err := os.Open("examples/prompts/cases.jsonl")
	if err != nil {
		fail(err)
	}
	defer file.Close()
	failed, total := 0, 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var sample corpusCase
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			fail(err)
		}
		total++
		original := score.Evaluate(sample.Prompt)
		improved := cli.LocalImprove(sample.Prompt, nil, nil)
		ok := original.Score >= sample.ExpectedScore[0] && original.Score <= sample.ExpectedScore[1] && len(cli.LocalQuestions(original)) <= 2
		for _, want := range sample.MustKeep {
			if concrete(want) && !keepsConcreteFacts(improved, want) {
				ok = false
			}
		}
		for _, forbidden := range sample.MustNotInvent {
			if !contains(sample.Prompt, forbidden) && contains(improved, forbidden) {
				ok = false
			}
		}
		if ok {
			fmt.Printf("PASS %s\n", sample.ID)
		} else {
			failed++
			fmt.Printf("FAIL %s score=%d questions=%d\n", sample.ID, original.Score, len(cli.LocalQuestions(original)))
		}
	}
	if err := scanner.Err(); err != nil {
		fail(err)
	}
	fmt.Printf("%d/%d passed\n", total-failed, total)
	if failed != 0 {
		os.Exit(1)
	}
}

func concrete(value string) bool { return strings.ContainsAny(value, "0123456789`./") }
func contains(value, part string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(part))
}
func keepsConcreteFacts(value, phrase string) bool {
	for _, word := range strings.Fields(phrase) {
		if concrete(word) && !contains(value, strings.Trim(word, ",.;:")) {
			return false
		}
	}
	return true
}
func fail(err error) { fmt.Fprintln(os.Stderr, "prompttest:", err); os.Exit(1) }
