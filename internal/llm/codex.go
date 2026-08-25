package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/quality"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

const (
	DefaultCodexModel     = "gpt-5.6-terra"
	DefaultCodexReasoning = "medium"
	codexRewriteSchema    = `{"type":"object","properties":{"understood_task":{"type":"string"},"improved_prompt":{"type":"string"}},"required":["understood_task","improved_prompt"],"additionalProperties":false}`
)

type CodexUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

// ImproveWithCodex rewrites one prompt with the user's existing Codex login.
func ImproveWithCodex(ctx context.Context, prompt, chatContext string, questions, answers []string, model, reasoning string) (Assessment, CodexUsage, error) {
	if strings.TrimSpace(model) == "" {
		model = DefaultCodexModel
	}
	if strings.TrimSpace(reasoning) == "" {
		reasoning = DefaultCodexReasoning
	}
	input, required, err := BuildRewriteInput(prompt, chatContext, questions, answers)
	if err != nil {
		return Assessment{}, CodexUsage{}, err
	}
	tempDir, err := os.MkdirTemp("", "promptpatch-codex-")
	if err != nil {
		return Assessment{}, CodexUsage{}, err
	}
	defer os.RemoveAll(tempDir)
	schemaPath := filepath.Join(tempDir, "schema.json")
	if err := os.WriteFile(schemaPath, []byte(codexRewriteSchema), 0600); err != nil {
		return Assessment{}, CodexUsage{}, err
	}
	instruction := RewriteRubric + "\n- Araç çağırma, dosya veya ağ erişimi yapma; yalnızca verilen JSON girdisini yeniden yaz.\n\nGirdi:\n" + input
	args := []string{
		"exec", "--ephemeral", "--json", "--skip-git-repo-check", "--ignore-user-config",
		"--sandbox", "read-only", "-C", tempDir, "-m", model,
		"-c", `model_reasoning_effort="` + reasoning + `"`, "--output-schema", schemaPath, instruction,
	}
	stdout, err := exec.CommandContext(ctx, "codex", args...).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) > 0 {
			return Assessment{}, CodexUsage{}, fmt.Errorf("Codex çağrısı başarısız: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return Assessment{}, CodexUsage{}, fmt.Errorf("Codex çağrısı başarısız: %w", err)
	}
	message, usage, err := parseCodexEvents(stdout)
	if err != nil {
		return Assessment{}, usage, err
	}
	var response struct {
		UnderstoodTask string `json:"understood_task"`
		ImprovedPrompt string `json:"improved_prompt"`
	}
	if err := json.Unmarshal([]byte(message), &response); err != nil {
		return Assessment{}, usage, fmt.Errorf("Codex çıktısı çözümlenemedi: %w", err)
	}
	if strings.TrimSpace(response.UnderstoodTask) == "" {
		return Assessment{}, usage, fmt.Errorf("Codex görevi yorumlamadı")
	}
	candidate := preserveConstraints(prompt, cleanModelRewrite(response.ImprovedPrompt))
	if issues := quality.RewriteIssues(prompt, input, candidate, required); len(issues) > 0 {
		return Assessment{}, usage, fmt.Errorf("Codex çıktısı kalite kontrolünden geçmedi: %s", strings.Join(issues, "; "))
	}
	original, improved := score.Evaluate(prompt), score.Evaluate(candidate)
	return Assessment{
		Criteria: original.Criteria, Score: original.Score, ImprovedPrompt: candidate,
		ImprovedCriteria: improved.Criteria, ImprovedScore: improved.Score, QualityStatus: "passed",
	}, usage, nil
}

func parseCodexEvents(data []byte) (string, CodexUsage, error) {
	var message string
	var usage CodexUsage
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
			Usage CodexUsage `json:"usage"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			message = event.Item.Text
		}
		if event.Type == "turn.completed" {
			usage = event.Usage
		}
	}
	if err := scanner.Err(); err != nil {
		return "", usage, err
	}
	if message == "" {
		return "", usage, fmt.Errorf("Codex son mesaj üretmedi")
	}
	return message, usage, nil
}
