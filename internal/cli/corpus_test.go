package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

type corpusCase struct {
	ID            string   `json:"id"`
	Prompt        string   `json:"prompt"`
	Questions     []string `json:"questions"`
	MustKeep      []string `json:"must_keep"`
	MustNotInvent []string `json:"must_not_invent"`
}

func TestLocalFallbackCorpus(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "examples", "prompts", "cases.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var sample corpusCase
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		questions := LocalQuestions(score.Evaluate(sample.Prompt))
		if len(questions) > 2 || len(sample.Questions) == 0 && len(questions) != 0 {
			t.Errorf("%s: questions=%d", sample.ID, len(questions))
		}
		improved := LocalImprove(sample.Prompt, nil, nil)
		for _, want := range sample.MustKeep {
			if isConcrete(want) && !keepsConcreteFacts(improved, want) {
				t.Errorf("%s: missing %q", sample.ID, want)
			}
		}
		for _, forbidden := range sample.MustNotInvent {
			if !containsFolded(sample.Prompt, forbidden) && containsFolded(improved, forbidden) {
				t.Errorf("%s: invented %q", sample.ID, forbidden)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func containsFolded(value, part string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(part))
}

func containsMeaning(value, phrase string) bool {
	value, phrase = fold(value), fold(phrase)
	if strings.Contains(value, phrase) {
		return true
	}
	for _, word := range strings.Fields(phrase) {
		if len([]rune(word)) < 4 {
			continue
		}
		if strings.Contains(value, word) {
			continue
		}
		root := string([]rune(word)[:4])
		if strings.Contains(value, root) {
			continue
		}
		if word == "koru" && strings.Contains(value, "bozma") {
			continue
		}
		return false
	}
	return true
}

func fold(value string) string {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer("ş", "s", "ı", "i", "ğ", "g", "ü", "u", "ö", "o", "ç", "c")
	return replacer.Replace(value)
}

func isConcrete(value string) bool {
	return strings.ContainsAny(value, "0123456789`./")
}

func keepsConcreteFacts(value, phrase string) bool {
	for _, word := range strings.Fields(phrase) {
		if isConcrete(word) && !containsFolded(value, strings.Trim(word, ",.;:")) {
			return false
		}
	}
	return true
}
