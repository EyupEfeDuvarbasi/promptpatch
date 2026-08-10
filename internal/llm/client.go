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
	Ollama    Provider = "ollama"
)

const (
	defaultOpenAIModel    = "gpt-5.6-terra"
	defaultGeminiModel    = "gemini-3.6-flash"
	defaultAnthropicModel = "claude-sonnet-4-20250514"
	defaultOllamaModel    = "qwen2.5:7b"
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
	if apiKey == "" && provider != Ollama {
		return Client{}, fmt.Errorf("%s API key is missing", provider)
	}
	switch provider {
	case OpenAI:
		return Client{Provider: provider, APIKey: apiKey, Model: defaultOpenAIModel, URL: "https://api.openai.com/v1/responses"}, nil
	case Gemini:
		return Client{Provider: provider, APIKey: apiKey, Model: defaultGeminiModel, URL: "https://generativelanguage.googleapis.com/v1beta/interactions"}, nil
	case Anthropic:
		return Client{Provider: provider, APIKey: apiKey, Model: defaultAnthropicModel, URL: "https://api.anthropic.com/v1/messages"}, nil
	case Ollama:
		return Client{Provider: provider, Model: defaultOllamaModel, URL: "http://127.0.0.1:11434/api/generate"}, nil
	default:
		return Client{}, fmt.Errorf("unsupported provider %q", provider)
	}
}

// Assess returns semantic criterion scores and, only when essential context is missing, up to two questions.
func (c Client) Assess(ctx context.Context, prompt string) (Assessment, error) {
	if strings.TrimSpace(prompt) == "" {
		return Assessment{}, fmt.Errorf("prompt is empty")
	}
	return c.assess(ctx, prompt, rubric)
}

// Improve scores only the original prompt and rewrites it using the supplied answers.
func (c Client) Improve(ctx context.Context, prompt string, questions, answers []string) (Assessment, error) {
	if strings.TrimSpace(prompt) == "" {
		return Assessment{}, fmt.Errorf("prompt is empty")
	}
	if c.Provider == Ollama {
		return c.improveOllama(ctx, prompt, questions, answers)
	}
	context := make([]map[string]string, 0, len(questions))
	for i, question := range questions {
		answer := ""
		if i < len(answers) {
			answer = answers[i]
		}
		context = append(context, map[string]string{"question": question, "answer": answer})
	}
	bundle, err := json.Marshal(map[string]any{"original_prompt": prompt, "additional_context": context})
	if err != nil {
		return Assessment{}, err
	}
	assessment, err := c.assess(ctx, string(bundle), rewriteRubric)
	if err != nil {
		return Assessment{}, err
	}
	if len(assessment.Questions) != 0 || assessment.ImprovedPrompt == "" {
		return Assessment{}, fmt.Errorf("model iyileştirilmiş prompt üretmedi (sorular: %s, çıktı: %d karakter)", strings.Join(assessment.Questions, " | "), len(assessment.ImprovedPrompt))
	}
	return assessment, nil
}

func (c Client) improveOllama(ctx context.Context, prompt string, questions, answers []string) (Assessment, error) {
	parts := []string{"Özgün görev:\n" + prompt}
	for i, answer := range answers {
		if answer != "" && i < len(questions) {
			parts = append(parts, "Doğrulanmış bilgi ("+questions[i]+"): "+answer)
		}
	}
	body, err := json.Marshal(map[string]any{
		"model": c.Model, "system": ollamaRewriteRubric, "prompt": strings.Join(parts, "\n\n"),
		"format": map[string]any{"type": "object", "properties": map[string]any{"improved_prompt": map[string]string{"type": "string"}}, "required": []string{"improved_prompt"}},
		"stream": false, "keep_alive": "5m", "options": map[string]any{"temperature": 0, "num_predict": 120},
	})
	if err != nil {
		return Assessment{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return Assessment{}, err
	}
	req.Header.Set("Content-Type", "application/json")
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
		return Assessment{}, fmt.Errorf("Ollama API returned %s: %s", res.Status, apiMessage(responseBody))
	}
	var response struct {
		Response string `json:"response"`
	}
	var rewritten struct {
		ImprovedPrompt string `json:"improved_prompt"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return Assessment{}, fmt.Errorf("yerel model yanıtı çözümlenemedi: %w", err)
	}
	if err := json.Unmarshal([]byte(response.Response), &rewritten); err != nil {
		return Assessment{}, fmt.Errorf("yerel model yanıtı çözümlenemedi")
	}
	if strings.TrimSpace(rewritten.ImprovedPrompt) == "" {
		return Assessment{}, fmt.Errorf("yerel model iyileştirilmiş prompt üretmedi")
	}
	original := score.Evaluate(prompt)
	improved := score.Evaluate(rewritten.ImprovedPrompt)
	return Assessment{Criteria: original.Criteria, Score: original.Score, ImprovedPrompt: strings.TrimSpace(rewritten.ImprovedPrompt), ImprovedCriteria: improved.Criteria, ImprovedScore: improved.Score}, nil
}

func (c Client) assess(ctx context.Context, input, instructions string) (Assessment, error) {
	body, err := c.requestBody(input, instructions)
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
	} else if c.Provider == Anthropic {
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

func (c Client) requestBody(prompt, instructions string) ([]byte, error) {
	if c.Model == "" {
		return nil, fmt.Errorf("model is missing")
	}
	if c.Provider == OpenAI {
		return json.Marshal(map[string]any{
			"model":        c.Model,
			"instructions": instructions,
			"input":        prompt,
			"text": map[string]any{"format": map[string]any{
				"type": "json_schema", "name": "prompt_assessment", "strict": true, "schema": assessmentSchema(),
			}},
		})
	}
	if c.Provider == Gemini {
		return json.Marshal(map[string]any{
			"model": c.Model,
			"input": instructions + "\n\nPROMPT:\n" + prompt,
			"response_format": map[string]any{
				"type": "text", "mime_type": "application/json", "schema": assessmentSchema(),
			},
		})
	}
	if c.Provider == Anthropic {
		return json.Marshal(map[string]any{
			"model": c.Model, "max_tokens": 1000, "system": instructions,
			"messages": []map[string]any{{"role": "user", "content": prompt}},
		})
	}
	if c.Provider == Ollama {
		return json.Marshal(map[string]any{
			"model": c.Model, "system": instructions, "prompt": prompt, "format": assessmentSchema(),
			"stream": false, "think": false, "keep_alive": "5m", "options": map[string]any{"temperature": 0},
		})
	}
	return nil, fmt.Errorf("unsupported provider %q", c.Provider)
}

func (c Client) decodeResponse(body []byte) (Assessment, error) {
	var response struct {
		OutputText string `json:"output_text"`
		Response   string `json:"response"`
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
	if c.Provider == Ollama {
		return decodeAssessment(response.Response)
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
		{Name: "Amaç ve Görev Netliği", Score: raw.Clarity},
		{Name: "Bağlam ve Teknik Bilgi", Score: raw.Context},
		{Name: "Beklenen Sonuç", Score: raw.Specificity},
		{Name: "Kısıtlar ve Sınırlar", Score: raw.Constraints},
		{Name: "Belirsizlik / Uygulanabilirlik", Score: raw.Purpose},
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
		{Name: "Amaç ve Görev Netliği", Score: raw.ImprovedClarity},
		{Name: "Bağlam ve Teknik Bilgi", Score: raw.ImprovedContext},
		{Name: "Beklenen Sonuç", Score: raw.ImprovedSpecificity},
		{Name: "Kısıtlar ve Sınırlar", Score: raw.ImprovedConstraints},
		{Name: "Belirsizlik / Uygulanabilirlik", Score: raw.ImprovedPurpose},
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

const rewriteRubric = `Girdi bir JSON nesnesidir: original_prompt ve additional_context içindeki soru-cevap çiftleri. original_prompt'u temel alanlarda puanla. additional_context bilgilerini doğal biçimde birleştirerek Türkçe, gerçekten yeniden yazılmış bir geliştirici promptu üret. Soruları veya cevapları metnin sonuna ekleme; teknik bilgi uydurma. ÇIKTI KURALLARI: questions alanı mutlaka boş dizi [] olmalı; improved_prompt mutlaka boş olmayan yeniden yazılmış prompt olmalı. improved_* alanlarında yeni promptu puanla. Yalnızca şemaya uyan JSON döndür.`

const ollamaRewriteRubric = `Tek bir Türkçe geliştirici promptu yeniden yaz. Girdideki her doğrulanmış bilgi zorunludur; hiçbirini atlama veya özetleyip zayıflatma. Bilgileri tek, doğrudan kullanılabilir görev promptuna doğal biçimde birleştir. En fazla dört kısa cümle yaz. Markdown başlığı (#), madde işareti, soru-cevap biçimi ve "soru", "cevap", "doğrulanmış bilgi" veya "ek bilgi" ifadelerini kullanma. Teknik ayrıntı uydurma, çözüm veya kod verme. improved_prompt alanında yalnızca prompt yer almalı.`

func average(criteria []score.Criterion) int {
	total := 0
	for _, criterion := range criteria {
		total += criterion.Score
	}
	return total / len(criteria)
}
