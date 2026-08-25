package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

type corpusCase struct {
	ID         string   `json:"id"`
	Tier       string   `json:"tier"`
	Kind       string   `json:"kind"`
	Prompt     string   `json:"prompt"`
	Context    string   `json:"chat_context"`
	MustKeep   []string `json:"must_keep"`
	MustNotAdd []string `json:"must_not_add"`
}

type result struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Model         string         `json:"model"`
	Reasoning     string         `json:"reasoning"`
	Passed        bool           `json:"passed"`
	Issues        []string       `json:"issues,omitempty"`
	OriginalScore int            `json:"original_score"`
	ImprovedScore int            `json:"improved_score"`
	DurationMS    int64          `json:"duration_ms"`
	Usage         llm.CodexUsage `json:"usage"`
	Improved      string         `json:"improved_prompt,omitempty"`
	Error         string         `json:"error,omitempty"`
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
		item := run(ctx, *model, *reasoning, sample)
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

func run(ctx context.Context, model, reasoning string, sample corpusCase) result {
	started := time.Now()
	item := result{ID: sample.ID, Kind: sample.Kind, Model: model, Reasoning: reasoning, OriginalScore: score.Evaluate(sample.Prompt).Score, DurationMS: time.Since(started).Milliseconds()}
	assessment, used, err := llm.ImproveWithCodex(ctx, sample.Prompt, sample.Context, nil, nil, model, reasoning)
	item.DurationMS = time.Since(started).Milliseconds()
	item.Usage = used
	if err != nil {
		item.Error = err.Error()
		return item
	}
	input, _, err := llm.BuildRewriteInput(sample.Prompt, sample.Context, nil, nil)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	item.Improved = assessment.ImprovedPrompt
	item.ImprovedScore = assessment.ImprovedScore
	for _, forbidden := range sample.MustNotAdd {
		if !containsText(input, forbidden) && containsText(item.Improved, forbidden) {
			item.Issues = append(item.Issues, "kaynakta olmayan yasaklı ekleme: "+forbidden)
		}
	}
	item.Passed = len(item.Issues) == 0
	return item
}

func containsText(value, part string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(part))
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
