// Package llm evaluates prompt semantics through OpenAI or Gemini.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"

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
	required := requiredFacts(prompt, answers)
	input := strings.Join(parts, "\n\n")
	if len(required) > 0 {
		input += "\n\nYeni promptta aynen bulunması zorunlu ifadeler: " + quoteFacts(required)
	}
	rewritten, err := c.ollamaRewrite(ctx, input)
	if err != nil {
		return Assessment{}, err
	}
	if !genuineRewrite(prompt, rewritten) {
		return Assessment{}, fmt.Errorf("yerel model özgün promptu yeniden yazmadı")
	}
	rewritten = preserveConstraints(prompt, rewritten)
	if missing := missingFacts(rewritten, required); len(missing) > 0 {
		rewritten = addMissingFacts(rewritten, missing, answers)
	}
	original := score.Evaluate(prompt)
	improved := score.Evaluate(rewritten)
	return Assessment{Criteria: original.Criteria, Score: original.Score, ImprovedPrompt: rewritten, ImprovedCriteria: improved.Criteria, ImprovedScore: improved.Score}, nil
}

func (c Client) ollamaRewrite(ctx context.Context, input string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model": c.Model, "system": ollamaRewriteRubric, "prompt": input,
		"format": map[string]any{"type": "object", "properties": map[string]any{"improved_prompt": map[string]string{"type": "string"}}, "required": []string{"improved_prompt"}},
		"stream": false, "keep_alive": "5m", "options": map[string]any{"temperature": 0, "num_predict": 320},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Ollama API returned %s: %s", res.Status, apiMessage(responseBody))
	}
	var response struct {
		Response string `json:"response"`
	}
	var rewritten struct {
		ImprovedPrompt string `json:"improved_prompt"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("yerel model yanıtı çözümlenemedi: %w", err)
	}
	if err := json.Unmarshal([]byte(response.Response), &rewritten); err != nil {
		return "", fmt.Errorf("yerel model yanıtı çözümlenemedi")
	}
	improvedPrompt := strings.TrimSpace(rewritten.ImprovedPrompt)
	if improvedPrompt == "" {
		return "", fmt.Errorf("yerel model iyileştirilmiş prompt üretmedi")
	}
	return improvedPrompt, nil
}

var (
	wordPattern         = regexp.MustCompile(`[\p{L}\p{N}_./-]+`)
	capacityTypePattern = regexp.MustCompile(`(?i)(\d+\s*(?:kb|mb|gb|tb))\s+(?:ram|bellek|hafıza|disk(?:\s+kapasitesi)?|depolama(?:\s+kapasitesi)?)`)
)

// requiredFacts keeps concrete model names, numbers, units and answered details from being lost in a rewrite.
func requiredFacts(prompt string, answers []string) []string {
	facts := make([]string, 0, len(answers)+4)
	for _, answer := range answers {
		if answer = strings.TrimSpace(answer); answer != "" {
			facts = append(facts, answer)
		}
	}
	words := wordPattern.FindAllString(prompt, -1)
	for i, word := range words {
		if !containsDigit(word) {
			continue
		}
		fact := word
		if i >= 3 && i+1 < len(words) && isUnit(words[i+1]) {
			fact = strings.Join(words[i-3:i+2], " ")
		}
		facts = append(facts, fact)
	}
	return uniqueFacts(facts)
}

func isUnit(value string) bool {
	switch strings.ToLower(value) {
	case "kb", "mb", "gb", "tb", "hz", "fps", "ms", "px":
		return true
	}
	return false
}

func containsDigit(value string) bool {
	for _, r := range value {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func uniqueFacts(facts []string) []string {
	seen := make(map[string]bool, len(facts))
	result := make([]string, 0, len(facts))
	for _, fact := range facts {
		key := strings.ToLower(strings.TrimSpace(fact))
		if key != "" && !seen[key] {
			seen[key] = true
			result = append(result, strings.TrimSpace(fact))
		}
	}
	return result
}

func quoteFacts(facts []string) string {
	quoted := make([]string, len(facts))
	for i, fact := range facts {
		quoted[i] = `"` + fact + `"`
	}
	return strings.Join(quoted, ", ")
}

func missingFacts(candidate string, required []string) []string {
	candidate = strings.ToLower(candidate)
	missing := make([]string, 0)
	for _, fact := range required {
		if !strings.Contains(candidate, strings.ToLower(fact)) {
			missing = append(missing, fact)
		}
	}
	return missing
}

func genuineRewrite(original, candidate string) bool {
	original = normalizeText(original)
	candidate = normalizeText(candidate)
	return candidate != "" && candidate != original && !strings.HasPrefix(candidate, original)
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func addMissingFacts(prompt string, missing, answers []string) string {
	answer := make(map[string]bool, len(answers))
	for _, value := range answers {
		answer[strings.ToLower(strings.TrimSpace(value))] = true
	}
	for _, fact := range missing {
		if answer[strings.ToLower(fact)] {
			prompt += " Çıktıyı " + fact + " olarak sun."
		} else {
			prompt += " Şu somut gereksinimi koru: " + fact + "."
		}
	}
	return prompt
}

// preserveConstraints keeps explicit user constraints out of the model's paraphrasing path.
func preserveConstraints(source, rewritten string) string {
	constraints := sourceConstraints(source)
	if len(constraints) == 0 {
		return removeUnsupportedCapacityTypes(source, strings.ReplaceAll(rewritten, "Doğrulanmış bilgi ", ""))
	}
	lines := strings.Split(strings.ReplaceAll(rewritten, "Doğrulanmış bilgi ", ""), "\n")
	sourceText := foldTurkish(source)
	for i, line := range lines {
		lower := foldTurkish(line)
		if strings.Contains(sourceText, "acik kaynak") && strings.Contains(sourceText, "yara") && strings.Contains(lower, "acik kaynak") && !strings.Contains(lower, "yararlan") {
			lines[i] = removeOpenSourceLead(line)
		}
	}
	return removeUnsupportedCapacityTypes(source, replaceSection(strings.Join(lines, "\n"), "Kısıtlar", bulletList(constraints)))
}

func removeUnsupportedCapacityTypes(source, rewritten string) string {
	source = foldTurkish(source)
	if strings.Contains(source, "ram") || strings.Contains(source, "bellek") || strings.Contains(source, "hafiza") || strings.Contains(source, "disk") || strings.Contains(source, "depolama") {
		return rewritten
	}
	return capacityTypePattern.ReplaceAllString(rewritten, "$1")
}

func removeOpenSourceLead(line string) string {
	lower := strings.ToLower(line)
	marker := "yola çıkarak"
	if end := strings.Index(lower, marker); end >= 0 {
		return strings.TrimSpace(strings.TrimLeft(line[end+len(marker):], ", "))
	}
	return ""
}

func sourceConstraints(source string) []string {
	lower := foldTurkish(source)
	constraints := []string{}
	if strings.Contains(lower, "onceki kod") && (strings.Contains(lower, "referans almadan") || strings.Contains(lower, "referans alma")) {
		constraints = append(constraints, "Önceki kodları referans alma.")
	}
	if strings.Contains(lower, "acik kaynak") && strings.Contains(lower, "yara") {
		constraints = append(constraints, "Açık kaynak kodlardan yararlan.")
	}
	if strings.Contains(lower, "fazlara bol") {
		constraints = append(constraints, "Çözümü aşamalara böl.")
	}
	if strings.Contains(lower, "agile") {
		constraints = append(constraints, "Agile yaklaşımı izle.")
	}
	if strings.Contains(lower, "her faz") && (strings.Contains(lower, "sonunda") || strings.Contains(lower, "sonundan")) {
		constraints = append(constraints, "Her aşamanın sonunda görünür ve doğrulanabilir bir sonuç sun.")
	}
	return constraints
}

func foldTurkish(value string) string {
	replacer := strings.NewReplacer("ç", "c", "Ç", "c", "ğ", "g", "Ğ", "g", "ı", "i", "I", "i", "İ", "i", "ö", "o", "Ö", "o", "ş", "s", "Ş", "s", "ü", "u", "Ü", "u")
	return replacer.Replace(strings.ToLower(value))
}

func bulletList(items []string) string {
	lines := make([]string, len(items))
	for i, item := range items {
		lines[i] = "- " + item
	}
	return strings.Join(lines, "\n")
}

func replaceSection(value, title, body string) string {
	header := "## " + title
	start := strings.Index(value, header)
	if start < 0 {
		return strings.TrimSpace(value) + "\n\n" + header + "\n" + body
	}
	afterHeader := start + len(header)
	next := strings.Index(value[afterHeader:], "\n## ")
	if next < 0 {
		return strings.TrimSpace(value[:afterHeader]) + "\n" + body
	}
	next += afterHeader
	return strings.TrimSpace(value[:afterHeader]) + "\n" + body + "\n\n" + strings.TrimSpace(value[next:])
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

const ollamaRewriteRubric = `Bu bir PROMPT DÜZENLEME işlemidir. Girdideki görevi çözme, araştırma yapma veya plan üretme. Yalnızca kullanıcının başka bir AI'a göndereceği yeniden yazılmış istek metnini üret.

Önce özgün metindeki görevleri, bağlamı, kısıtları, başarı ölçütlerini ve teslimatı ayır; sonra bunları daha açık yaz. Yazım ve dil bilgisini düzeltebilirsin, fakat anlamı değiştiremezsin. Teknik ayrıntı uydurma veya belirsiz bilgiyi teknik bir gerçeğe dönüştürme: kaynakta yalnızca bir kapasite yazıyorsa türünü (RAM, disk vb.) ekleme. Her kısıtın olumlu/olumsuz anlamını aynen koru; farklı kısıtları birleştirme, tersine çevirme veya kaynakta olmayan kısıt ekleme.

"Zorunlu ifadeler" verildiyse her birini harf harfine koru. Özgün görevdeki somut adları, sayıları, birimleri, teknolojileri, dosya adlarını ve kullanıcı cevaplarını asla çıkarma veya genelleştirme.

Yalnızca aşağıdaki Markdown yapısında, kısa ve doğrudan bir prompt yaz. Kaynakta bilgi yoksa bölüm ekleme:
## Görev
## Bağlam
## Kısıtlar
## Başarı ölçütleri
## Teslimat

Soru-cevap biçimi, açıklama, çözüm veya kod yazma. improved_prompt alanında yalnızca bu prompt yer almalı.`

func average(criteria []score.Criterion) int {
	total := 0
	for _, criterion := range criteria {
		total += criterion.Score
	}
	return total / len(criteria)
}
