// Package llm evaluates prompt semantics through OpenAI or Gemini.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

type Provider string

const (
	OpenAI    Provider = "openai"
	Gemini    Provider = "gemini"
	Anthropic Provider = "anthropic"
)

const (
	defaultOpenAIModel    = "gpt-5.6-terra"
	defaultGeminiModel    = "gemini-3.6-flash"
	defaultAnthropicModel = "claude-sonnet-4-20250514"
)

type Client struct {
	Provider   Provider
	APIKey     string
	Model      string
	URL        string
	HTTPClient *http.Client
}

type Assessment struct {
	Criteria         []score.Criterion
	Score            int
	Questions        []string
	ImprovedPrompt   string
	ImprovedCriteria []score.Criterion
	ImprovedScore    int
}

type assessmentJSON struct {
	Clarity             int      `json:"clarity"`
	Specificity         int      `json:"specificity"`
	Context             int      `json:"context"`
	Constraints         int      `json:"constraints"`
	Purpose             int      `json:"purpose"`
	Questions           []string `json:"questions"`
	ImprovedPrompt      string   `json:"improved_prompt"`
	ImprovedClarity     int      `json:"improved_clarity"`
	ImprovedSpecificity int      `json:"improved_specificity"`
	ImprovedContext     int      `json:"improved_context"`
	ImprovedConstraints int      `json:"improved_constraints"`
	ImprovedPurpose     int      `json:"improved_purpose"`
}

func New(provider Provider, apiKey string) (Client, error) {
	if apiKey == "" {
		return Client{}, fmt.Errorf("%s API key is missing", provider)
	}
	switch provider {
	case OpenAI:
		return Client{Provider: provider, APIKey: apiKey, Model: defaultOpenAIModel, URL: "https://api.openai.com/v1/responses"}, nil
	case Gemini:
		return Client{Provider: provider, APIKey: apiKey, Model: defaultGeminiModel, URL: "https://generativelanguage.googleapis.com/v1beta/interactions"}, nil
	case Anthropic:
		return Client{Provider: provider, APIKey: apiKey, Model: defaultAnthropicModel, URL: "https://api.anthropic.com/v1/messages"}, nil
	default:
		return Client{}, fmt.Errorf("unsupported provider %q", provider)
	}
}

// Assess returns semantic criterion scores and, only when essential context is missing, up to two questions.
func (c Client) Assess(ctx context.Context, prompt string) (Assessment, error) {
	if strings.TrimSpace(prompt) == "" {
		return Assessment{}, fmt.Errorf("prompt is empty")
	}
	body, err := c.requestBody(prompt)
	if err != nil {
		return Assessment{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return Assessment{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Provider == OpenAI {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	} else if c.Provider == Gemini {
		req.Header.Set("x-goog-api-key", c.APIKey)
	} else {
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return Assessment{}, err
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return Assessment{}, err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return Assessment{}, fmt.Errorf("%s API returned %s: %s", c.Provider, res.Status, apiMessage(responseBody))
	}

	return c.decodeResponse(responseBody)
}

func (c Client) requestBody(prompt string) ([]byte, error) {
	if c.Model == "" {
		return nil, fmt.Errorf("model is missing")
	}
	if c.Provider == OpenAI {
		return json.Marshal(map[string]any{
			"model":        c.Model,
			"instructions": rubric,
			"input":        prompt,
			"text": map[string]any{"format": map[string]any{
				"type": "json_schema", "name": "prompt_assessment", "strict": true, "schema": assessmentSchema(),
			}},
		})
	}
	if c.Provider == Gemini {
		return json.Marshal(map[string]any{
			"model": c.Model,
			"input": rubric + "\n\nPROMPT:\n" + prompt,
			"response_format": map[string]any{
				"type": "text", "mime_type": "application/json", "schema": assessmentSchema(),
			},
		})
	}
	if c.Provider == Anthropic {
		return json.Marshal(map[string]any{
			"model": c.Model, "max_tokens": 1000, "system": rubric,
			"messages": []map[string]any{{"role": "user", "content": prompt}},
		})
	}
	return nil, fmt.Errorf("unsupported provider %q", c.Provider)
}

func (c Client) decodeResponse(body []byte) (Assessment, error) {
	var response struct {
		OutputText string `json:"output_text"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Assessment{}, fmt.Errorf("decode %s response: %w", c.Provider, err)
	}
	if c.Provider == Anthropic {
		for _, content := range response.Content {
			if content.Type == "text" {
				return decodeAssessment(content.Text)
			}
		}
		return Assessment{}, fmt.Errorf("Anthropic response has no text content")
	}
	return decodeAssessment(response.OutputText)
}

func decodeAssessment(text string) (Assessment, error) {
	var raw assessmentJSON
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return Assessment{}, fmt.Errorf("decode assessment: %w", err)
	}
	if len(raw.Questions) > 2 {
		return Assessment{}, fmt.Errorf("assessment returned more than two questions")
	}
	criteria := []score.Criterion{
		{Name: "Netlik", Score: raw.Clarity},
		{Name: "Spesifiklik", Score: raw.Specificity},
		{Name: "Bağlam Yeterliliği", Score: raw.Context},
		{Name: "Kısıtlar ve Çıktı", Score: raw.Constraints},
		{Name: "Amaç ve Başarı Ölçütü", Score: raw.Purpose},
	}
	total := 0
	for _, criterion := range criteria {
		if criterion.Score < 0 || criterion.Score > 100 {
			return Assessment{}, fmt.Errorf("invalid %s score", criterion.Name)
		}
		total += criterion.Score
	}
	assessment := Assessment{Criteria: criteria, Score: total / len(criteria), Questions: raw.Questions, ImprovedPrompt: raw.ImprovedPrompt}
	if raw.ImprovedPrompt == "" {
		return assessment, nil
	}
	improved := []score.Criterion{
		{Name: "Netlik", Score: raw.ImprovedClarity},
		{Name: "Spesifiklik", Score: raw.ImprovedSpecificity},
		{Name: "Bağlam Yeterliliği", Score: raw.ImprovedContext},
		{Name: "Kısıtlar ve Çıktı", Score: raw.ImprovedConstraints},
		{Name: "Amaç ve Başarı Ölçütü", Score: raw.ImprovedPurpose},
	}
	for _, criterion := range improved {
		if criterion.Score < 0 || criterion.Score > 100 {
			return Assessment{}, fmt.Errorf("invalid improved %s score", criterion.Name)
		}
	}
	assessment.ImprovedCriteria = improved
	assessment.ImprovedScore = average(improved)
	return assessment, nil
}

func assessmentSchema() map[string]any {
	integer := map[string]any{"type": "integer", "minimum": 0, "maximum": 100}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"clarity": integer, "specificity": integer, "context": integer, "constraints": integer, "purpose": integer,
			"questions":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 2},
			"improved_prompt":      map[string]any{"type": "string"},
			"improved_clarity":     integer,
			"improved_specificity": integer,
			"improved_context":     integer,
			"improved_constraints": integer,
			"improved_purpose":     integer,
		},
		"required": []string{"clarity", "specificity", "context", "constraints", "purpose", "questions", "improved_prompt", "improved_clarity", "improved_specificity", "improved_context", "improved_constraints", "improved_purpose"},
	}
}

func apiMessage(body []byte) string {
	var response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &response) == nil && response.Error.Message != "" {
		return response.Error.Message
	}
	return "request failed"
}

const rubric = `Evaluate this developer prompt on five criteria from 0 to 100: clarity, specificity, context, constraints/output, and purpose/success criteria. Do not penalize omitted file names or technologies unless they are needed by the request. Always score the input in the base fields. If essential information prevents a useful improvement, return at most two concise Turkish questions, an empty improved_prompt, and zero for every improved_* score. Otherwise return no questions, a Turkish improved_prompt, and score that improved prompt in every improved_* field. Return only JSON matching the schema.`

func average(criteria []score.Criterion) int {
	total := 0
	for _, criterion := range criteria {
		total += criterion.Score
	}
	return total / len(criteria)
}
