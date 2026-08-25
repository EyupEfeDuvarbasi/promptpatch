package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/quality"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

const schema = `{"type":"object","properties":{"understood_task":{"type":"string"},"improved_prompt":{"type":"string"}},"required":["understood_task","improved_prompt"],"additionalProperties":false}`

type corpusCase struct {
	ID         string   `json:"id"`
	Tier       string   `json:"tier"`
	Kind       string   `json:"kind"`
	Prompt     string   `json:"prompt"`
	Context    string   `json:"chat_context"`
	MustKeep   []string `json:"must_keep"`
	MustNotAdd []string `json:"must_not_add"`
}

type usage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

type result struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Model         string   `json:"model"`
	Reasoning     string   `json:"reasoning"`
	Passed        bool     `json:"passed"`
	Issues        []string `json:"issues,omitempty"`
	OriginalScore int      `json:"original_score"`
	ImprovedScore int      `json:"improved_score"`
	DurationMS    int64    `json:"duration_ms"`
	Usage         usage    `json:"usage"`
	Improved      string   `json:"improved_prompt,omitempty"`
	Error         string   `json:"error,omitempty"`
}

func main() {
	casesPath := flag.String("cases", "test/model-cases.jsonl", "JSONL corpus path")
	tier := flag.String("tier", "core", "core or all")
	model := flag.String("model", "gpt-5.6-terra", "Codex model")
	reasoning := flag.String("reasoning", "low", "Codex reasoning effort")
	outputPath := flag.String("out", "", "optional JSONL result path")
	limit := flag.Int("limit", 0, "maximum case count")
	ids := flag.String("ids", "", "optional comma-separated case IDs")
	timeout := flag.Duration("timeout", 2*time.Minute, "timeout per case")
	flag.Parse()

	cases, err := loadCases(*casesPath, *tier, *limit)
	if err != nil {
		fail(err)
	}
	if *ids != "" {
		cases = selectCases(cases, strings.Split(*ids, ","))
	}
	tempDir, err := os.MkdirTemp("", "promptbench-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(tempDir)
	schemaPath := filepath.Join(tempDir, "schema.json")
	if err := os.WriteFile(schemaPath, []byte(schema), 0600); err != nil {
		fail(err)
	}

	var output *os.File
	if *outputPath != "" {
		output, err = os.Create(*outputPath)
		if err != nil {
			fail(err)
		}
		defer output.Close()
	}

	passed, inputTokens, cachedTokens, outputTokens, reasoningTokens := 0, 0, 0, 0, 0
	var elapsed int64
	for index, sample := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		item := run(ctx, tempDir, schemaPath, *model, *reasoning, sample)
		cancel()
		if item.Passed {
			passed++
		}
		inputTokens += item.Usage.InputTokens
		cachedTokens += item.Usage.CachedInputTokens
		outputTokens += item.Usage.OutputTokens
		reasoningTokens += item.Usage.ReasoningOutputTokens
		elapsed += item.DurationMS
		if output != nil {
			encoded, _ := json.Marshal(item)
			fmt.Fprintln(output, string(encoded))
		}
		fmt.Printf("[%d/%d] %s %s score=%d→%d tokens=%d/%d/%d %dms\n", index+1, len(cases), status(item), item.ID, item.OriginalScore, item.ImprovedScore, item.Usage.InputTokens, item.Usage.OutputTokens, item.Usage.ReasoningOutputTokens, item.DurationMS)
	}
	fmt.Printf("SUMMARY model=%s reasoning=%s passed=%d/%d input=%d cached=%d output=%d reasoning_tokens=%d duration_ms=%d\n", *model, *reasoning, passed, len(cases), inputTokens, cachedTokens, outputTokens, reasoningTokens, elapsed)
}

func selectCases(cases []corpusCase, ids []string) []corpusCase {
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[strings.TrimSpace(id)] = true
	}
	selected := make([]corpusCase, 0, len(ids))
	for _, sample := range cases {
		if wanted[sample.ID] {
			selected = append(selected, sample)
		}
	}
	return selected
}

func loadCases(path, tier string, limit int) ([]corpusCase, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var cases []corpusCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var sample corpusCase
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			return nil, fmt.Errorf("invalid case %d: %w", len(cases)+1, err)
		}
		if (tier == "core" || tier == "full") && sample.Tier != tier {
			continue
		}
		cases = append(cases, sample)
		if limit > 0 && len(cases) == limit {
			break
		}
	}
	return cases, scanner.Err()
}

func run(ctx context.Context, cwd, schemaPath, model, reasoning string, sample corpusCase) result {
	started := time.Now()
	input, required, err := llm.BuildRewriteInput(sample.Prompt, sample.Context, nil, nil)
	if err != nil {
		return result{ID: sample.ID, Kind: sample.Kind, Model: model, Reasoning: reasoning, Error: err.Error()}
	}
	prompt := llm.RewriteRubric + "\n- Araç çağırma, dosya veya ağ erişimi yapma; yalnızca verilen JSON girdisini yeniden yaz.\n\nGirdi:\n" + input
	args := []string{"exec", "--ephemeral", "--json", "--skip-git-repo-check", "--ignore-user-config", "--sandbox", "read-only", "-C", cwd, "-m", model, "-c", `model_reasoning_effort="` + reasoning + `"`, "--output-schema", schemaPath, prompt}
	command := exec.CommandContext(ctx, "codex", args...)
	stdout, err := command.Output()
	item := result{ID: sample.ID, Kind: sample.Kind, Model: model, Reasoning: reasoning, OriginalScore: score.Evaluate(sample.Prompt).Score, DurationMS: time.Since(started).Milliseconds()}
	if err != nil {
		item.Error = commandError(err)
		return item
	}
	text, used, err := parseEvents(stdout)
	item.Usage = used
	if err != nil {
		item.Error = err.Error()
		return item
	}
	var response struct {
		Improved string `json:"improved_prompt"`
	}
	if err := json.Unmarshal([]byte(text), &response); err != nil {
		item.Error = "invalid structured output: " + err.Error()
		return item
	}
	item.Improved = strings.TrimSpace(response.Improved)
	item.ImprovedScore = score.Evaluate(item.Improved).Score
	item.Issues = quality.RewriteIssues(sample.Prompt, input, item.Improved, required)
	for _, forbidden := range sample.MustNotAdd {
		if !containsText(input, forbidden) && containsText(item.Improved, forbidden) {
			item.Issues = append(item.Issues, "kaynakta olmayan yasaklı ekleme: "+forbidden)
		}
	}
	item.Passed = len(item.Issues) == 0
	return item
}

func parseEvents(data []byte) (string, usage, error) {
	var final string
	var used usage
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
			Usage usage `json:"usage"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			final = event.Item.Text
		}
		if event.Type == "turn.completed" {
			used = event.Usage
		}
	}
	if err := scanner.Err(); err != nil {
		return "", used, err
	}
	if final == "" {
		return "", used, fmt.Errorf("Codex final message missing")
	}
	return final, used, nil
}

func containsText(value, part string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(part))
}

func commandError(err error) string {
	if exit, ok := err.(*exec.ExitError); ok {
		return strings.TrimSpace(string(exit.Stderr))
	}
	return err.Error()
}

func status(item result) string {
	if item.Passed {
		return "PASS"
	}
	if item.Error != "" {
		return "ERROR"
	}
	return "FAIL"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "promptbench:", err)
	os.Exit(1)
}
