package score

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type corpusCase struct {
	ID            string   `json:"id"`
	Prompt        string   `json:"prompt"`
	ExpectedScore [2]int   `json:"expected_score"`
	Questions     []string `json:"questions"`
}

func TestPromptCorpusScoresAreStableAndCalibrated(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "prompts", "cases.jsonl")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var sample corpusCase
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		first, second := Evaluate(sample.Prompt), Evaluate(sample.Prompt)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("%s: non-deterministic score", sample.ID)
		}
		if first.Score < sample.ExpectedScore[0] || first.Score > sample.ExpectedScore[1] {
			t.Errorf("%s: score=%d, want %d..%d; criteria=%v", sample.ID, first.Score, sample.ExpectedScore[0], sample.ExpectedScore[1], first.Criteria)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
