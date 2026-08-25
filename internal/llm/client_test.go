package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const apiAssessment = `{"clarity":80,"specificity":70,"context":60,"constraints":50,"purpose":90,"questions":[],"improved_prompt":"Detaylı prompt","improved_clarity":90,"improved_specificity":80,"improved_context":70,"improved_constraints":80,"improved_purpose":100}`

func TestAssessSendsProviderAuthAndParsesOutput(t *testing.T) {
	for _, provider := range []Provider{OpenAI, Gemini, Anthropic, Ollama} {
		t.Run(string(provider), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if provider == OpenAI && r.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatal("missing OpenAI authorization")
				}
				if provider == Gemini && r.Header.Get("x-goog-api-key") != "test-key" {
					t.Fatal("missing Gemini API key")
				}
				if provider == Anthropic && r.Header.Get("x-api-key") != "test-key" {
					t.Fatal("missing Anthropic API key")
				}
				if _, err := io.ReadAll(r.Body); err != nil {
					t.Fatal(err)
				}
				w.Header().Set("Content-Type", "application/json")
				if provider == Anthropic {
					_, _ = w.Write([]byte(`{"content":[{"type":"text","text":` + strconv.Quote(apiAssessment) + `}]}`))
					return
				}
				if provider == Ollama {
					_, _ = w.Write([]byte(`{"response":` + strconv.Quote(apiAssessment) + `}`))
					return
				}
				_, _ = w.Write([]byte(`{"output_text":` + strconv.Quote(apiAssessment) + `}`))
			}))
			defer server.Close()

			client, err := New(provider, "test-key")
			if err != nil {
				t.Fatal(err)
			}
			client.URL = server.URL
			result, err := client.Assess(context.Background(), "parserı düzelt")
			if err != nil || result.Score != 70 || result.ImprovedPrompt == "" {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestImproveSendsAnswersAndReturnsRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal(body, &request); err != nil || !strings.Contains(request.Input, `"original_prompt":"şunu düzelt"`) || !strings.Contains(request.Input, `"src/parser.go"`) {
			t.Fatalf("request=%s", body)
		}
		_, _ = w.Write([]byte(`{"output_text":` + strconv.Quote(apiAssessment) + `}`))
	}))
	defer server.Close()
	client, err := New(OpenAI, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	client.URL = server.URL
	result, err := client.Improve(context.Background(), "şunu düzelt", []string{"Hangi dosya?"}, []string{"src/parser.go"})
	if err != nil || result.ImprovedPrompt != "Detaylı prompt" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestImproveSendsChatContextAsReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Input string `json:"input"`
		}
		if json.Unmarshal(body, &request) != nil || !strings.Contains(request.Input, `"chat_context":"Kullanıcı önce PostgreSQL dedi."`) {
			t.Fatalf("chat context missing: %s", body)
		}
		_, _ = w.Write([]byte(`{"output_text":` + strconv.Quote(apiAssessment) + `}`))
	}))
	defer server.Close()
	client, _ := New(OpenAI, "test-key")
	client.URL = server.URL
	if _, err := client.ImproveWithContext(context.Background(), "şunu düzelt", "Kullanıcı önce PostgreSQL dedi.", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestOllamaImproveReturnsRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(`{"understood_task":"Belirtilen dosyayı düzelt","improved_prompt":"src/parser.go dosyasını düzelt."}`) + `,"done":true,"done_reason":"stop"}`))
	}))
	defer server.Close()
	client, err := New(Ollama, "")
	if err != nil {
		t.Fatal(err)
	}
	client.URL = server.URL
	client.Model = "test-model"
	result, err := client.Improve(context.Background(), "şunu düzelt", []string{"Hangi dosya?"}, []string{"src/parser.go"})
	if err != nil || result.ImprovedPrompt != "src/parser.go dosyasını düzelt." {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestOllamaImproveUsesInstalledModelWhenDefaultIsMissing(t *testing.T) {
	var requestedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest"}]}`))
			return
		}
		var request struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		requestedModel = request.Model
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(`{"understood_task":"Belirtilen dosyayı düzelt","improved_prompt":"src/parser.go dosyasını düzelt."}`) + `,"done":true,"done_reason":"stop"}`))
	}))
	defer server.Close()
	client, err := New(Ollama, "")
	if err != nil {
		t.Fatal(err)
	}
	client.URL = server.URL + "/api/generate"
	result, err := client.Improve(context.Background(), "şunu düzelt", []string{"Hangi dosya?"}, []string{"src/parser.go"})
	if err != nil || result.ImprovedPrompt == "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if requestedModel != "llama3:latest" {
		t.Fatalf("model=%q", requestedModel)
	}
}

func TestInstalledOllamaModelPrefersQwenThenGemma(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"gemma3:4b"},{"name":"qwen2.5:7b"}]}`))
	}))
	defer server.Close()
	client, _ := New(Ollama, "")
	client.URL = server.URL + "/api/generate"
	model, err := client.InstalledOllamaModel(context.Background())
	if err != nil || model != "qwen2.5:7b" {
		t.Fatalf("model=%q err=%v", model, err)
	}
}

func TestOllamaImproveRetriesTruncatedOutputOnce(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var request struct {
			Prompt  string         `json:"prompt"`
			Options map[string]int `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Options["num_ctx"] != 8192 {
			t.Fatalf("options=%v", request.Options)
		}
		if attempts == 1 {
			if request.Options["num_predict"] != 1536 {
				t.Fatalf("first options=%v", request.Options)
			}
			_, _ = w.Write([]byte(`{"response":"{\"understood_task\":\"Plan oluştur\",\"improved_prompt\":\"yarım","done":true,"done_reason":"length"}`))
			return
		}
		if request.Options["num_predict"] != 2048 || !strings.Contains(request.Prompt, "token sınırında kesildi") {
			t.Fatalf("retry request=%#v", request)
		}
		payload := `{"understood_task":"İki projeyi birleştirme planı oluştur","improved_prompt":"PromptPatch ile PromptLens projelerini tek bir ürün hâline getirmek için eksiksiz ve uygulanabilir bir plan oluştur."}`
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(payload) + `,"done":true,"done_reason":"stop"}`))
	}))
	defer server.Close()
	client, _ := New(Ollama, "")
	client.URL, client.Model = server.URL, "test-model"
	result, err := client.ImproveWithContext(context.Background(), "PromptPatch ile PromptLens projelerini birleştirmek için plan oluştur.", "", nil, nil)
	if err != nil || result.QualityStatus != "corrected" || attempts != 2 {
		t.Fatalf("result=%#v attempts=%d err=%v", result, attempts, err)
	}
}

func TestOllamaImproveRejectsBothBadAttempts(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		payload := `{"understood_task":"Plan oluştur","improved_prompt":"Bu, oldukça karmaşık bir projedir. Başarılı olmak için 2 hafta ayır."}`
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(payload) + `,"done":true,"done_reason":"stop"}`))
	}))
	defer server.Close()
	client, _ := New(Ollama, "")
	client.URL, client.Model = server.URL, "test-model"
	_, err := client.ImproveWithContext(context.Background(), "PromptPatch ve PromptLens için plan oluştur.", "", nil, nil)
	if err == nil || attempts != 2 || !strings.Contains(err.Error(), "genel bir giriş") {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRequiredFactsPreserveAnswersAndTechnicalNumbers(t *testing.T) {
	facts := requiredFacts("promptpatchi PromptLens ile birleştir; src/parser.go ve jetson orin nano 8 gb üzerinde 10 kamera ve 20 fps hedefle", []string{"md dosyası"})
	for _, want := range []string{"md dosyası", "promptpatch", "promptlens", "src/parser.go", "jetson orin nano 8 gb", "10", "20"} {
		if !strings.Contains(strings.Join(facts, "|"), want) {
			t.Fatalf("facts=%q, missing %q", facts, want)
		}
	}
	if !genuineRewrite("şunu düzelt", "src/parser.go için boş girdi hatasını düzelt.") {
		t.Fatal("rewrite should be genuine")
	}
	if genuineRewrite("şunu düzelt", "şunu düzelt. md dosyası") {
		t.Fatal("an appended answer is not a rewrite")
	}
}

func TestPreserveConstraintsKeepsOpposedRulesSeparate(t *testing.T) {
	source := "önceki kodlardan referans almadan ve açık kaynak kodlardan yararlanarak fazlara böl, agile mantığında her faz sonunda görünür sonuç sun"
	modelOutput := "## Görev\nAçık kaynak kodlarından yola çıkarak plan hazırla.\n\n## Kısıtlar\n- Açık kaynak kullanma.\n\n## Teslimat\nmd dosyası"
	got := preserveConstraints(source, modelOutput)
	for _, want := range []string{"Önceki kodları referans alma.", "Açık kaynak kodlardan yararlan.", "Çözümü aşamalara böl.", "Agile yaklaşımı izle.", "Her aşamanın sonunda görünür ve doğrulanabilir bir sonuç sun."} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewrite=%q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "Açık kaynak kullanma") || strings.Contains(got, "Açık kaynak kodlarından yola") || !strings.Contains(got, "## Görev\nplan hazırla") {
		t.Fatalf("contradictory source use remains: %q", got)
	}
}

func TestSourceConstraintsToleratesTurkishTyping(t *testing.T) {
	got := strings.Join(sourceConstraints("onceki kodlardan referans almadan ve acik kaynak kodlardan yaralaranka fazlara bol"), "|")
	for _, want := range []string{"Önceki kodları referans alma.", "Açık kaynak kodlardan yararlan.", "Çözümü aşamalara böl."} {
		if !strings.Contains(got, want) {
			t.Fatalf("constraints=%q, missing %q", got, want)
		}
	}
}

func TestPreserveConstraintsRemovesUnsupportedCapacityType(t *testing.T) {
	got := preserveConstraints("jetson orin nano 8 gb üzerinde plan hazırla", "## Bağlam\nJetson Orin Nano 8 GB disk kapasitesi kullan.")
	if strings.Contains(strings.ToLower(got), "disk") || !strings.Contains(got, "8 GB") {
		t.Fatalf("rewrite=%q", got)
	}
}

func TestMissingFactsRejectsInsteadOfAppendingAnswers(t *testing.T) {
	missing := missingFacts("## Görev\nPlanı hazırla.", []string{"md dosyası", "Jetson Orin Nano 8 GB"})
	if strings.Join(missing, "|") != "md dosyası|Jetson Orin Nano 8 GB" {
		t.Fatalf("missing=%q", missing)
	}
}

func TestMissingFactsAcceptsMarkdownEquivalentForMDFile(t *testing.T) {
	for _, required := range []string{"md dosyası", "md dosyasını", "md dosyasına"} {
		if missing := missingFacts("## Teslimat\nRaporu Markdown formatında sun.", []string{required}); len(missing) != 0 {
			t.Fatalf("required=%q missing=%q", required, missing)
		}
	}
	if missing := missingFacts("## Teslimat\nRaporu sun.", []string{"md dosyası"}); len(missing) != 1 {
		t.Fatalf("missing=%q", missing)
	}
}

func TestLiveProviders(t *testing.T) {
	if os.Getenv("PROMPTCHECK_LIVE") != "1" {
		t.Skip("set PROMPTCHECK_LIVE=1 to call provider APIs")
	}
	for provider, keyName := range map[Provider]string{OpenAI: "OPENAI_API_KEY", Gemini: "GEMINI_API_KEY", Anthropic: "ANTHROPIC_API_KEY"} {
		t.Run(string(provider), func(t *testing.T) {
			if os.Getenv(keyName) == "" {
				t.Skip(keyName + " is not set")
			}
			client, err := New(provider, os.Getenv(keyName))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			assessment, err := client.Assess(ctx, "src/parser.go içindeki parseInput fonksiyonunu boş girdi için düzelt.")
			if err != nil || len(assessment.Criteria) != 5 {
				t.Fatalf("assessment failed: %v", err)
			}
		})
	}
}

func TestLiveOllamaRewrite(t *testing.T) {
	if os.Getenv("PROMPTCHECK_OLLAMA") != "1" {
		t.Skip("set PROMPTCHECK_OLLAMA=1 to call the local model")
	}
	client, err := New(Ollama, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	assessment, err := client.Improve(ctx, "şunu düzelt", []string{"Hangi dosya ve beklenen davranış nedir?"}, []string{"src/parser.go dosyasında boş girdi hata dönsün"})
	if err != nil {
		t.Fatal(err)
	}
	if assessment.ImprovedPrompt == "" || assessment.ImprovedPrompt == "şunu düzelt" {
		t.Fatalf("genuine rewrite missing: %#v", assessment)
	}
	if !strings.Contains(assessment.ImprovedPrompt, "src/parser.go") || !strings.Contains(strings.ToLower(assessment.ImprovedPrompt), "boş") {
		t.Fatalf("context missing from rewrite: %q", assessment.ImprovedPrompt)
	}
	if strings.Contains(assessment.ImprovedPrompt, "Soru:") || strings.Contains(assessment.ImprovedPrompt, "Cevap:") || strings.Contains(assessment.ImprovedPrompt, "Doğrulanmış bilgi") {
		t.Fatalf("raw Q&A leaked into rewrite: %q", assessment.ImprovedPrompt)
	}
	if strings.Contains(strings.ToLower(assessment.ImprovedPrompt), "here is") {
		t.Fatalf("model preamble leaked into rewrite: %q", assessment.ImprovedPrompt)
	}
}

func TestLiveOllamaPromptPatchPromptLensRegression(t *testing.T) {
	if os.Getenv("PROMPTCHECK_OLLAMA") != "1" {
		t.Skip("set PROMPTCHECK_OLLAMA=1 to call the local model")
	}
	client, err := New(Ollama, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	chatContext := "USER: PromptPatch ve PromptLens projelerinin uyumluluğunu araştır.\n\nUSER: API entegrasyonu sorun değil; iki projeyi tek bir ürün hâline getirmek istiyorum. Plan oluşturmadan önce uyumluluğu tekrar kontrol et."
	assessment, err := client.ImproveWithContext(ctx, "promptpatchi ve promptlensi birleştirmek için güzel bir plan oluşturalım", chatContext, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(assessment.ImprovedPrompt)
	if !strings.Contains(lower, "promptpatch") || !strings.Contains(lower, "promptlens") || strings.HasPrefix(lower, "bu, oldukça karmaşık") || strings.Contains(lower, "önceden belirlenen") || strings.Contains(lower, "zorunlu ifadeler") {
		t.Fatalf("rewrite=%q", assessment.ImprovedPrompt)
	}
	t.Logf("quality=%s rewrite=%s", assessment.QualityStatus, assessment.ImprovedPrompt)
}

func TestLiveOllamaPreservesOpposedConstraints(t *testing.T) {
	if os.Getenv("PROMPTCHECK_OLLAMA") != "1" {
		t.Skip("set PROMPTCHECK_OLLAMA=1 to call the local model")
	}
	client, err := New(Ollama, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	assessment, err := client.Improve(ctx, "önceki kodlardan referans almadan ve açık kaynak kodlardan yararlanarak Jetson Orin Nano 8 GB üzerinde 10 kameradan gelen videoyu aynı anda işleyecek, her kamera için 20 FPS hedefleyen aşamalı bir plan hazırla.", []string{"Beklenen davranış veya çıktı formatı nedir?"}, []string{"md dosyası"})
	if err != nil {
		t.Fatal(err)
	}
	output := strings.ToLower(assessment.ImprovedPrompt)
	for _, want := range []string{"önceki kod", "açık kaynak", "yararlan", "jetson orin nano 8 gb", "10", "20", "md dosyası"} {
		if !strings.Contains(output, want) {
			t.Fatalf("rewrite=%q, missing %q", assessment.ImprovedPrompt, want)
		}
	}
	if strings.Contains(output, "ram") {
		t.Fatalf("unsupported technical assumption in rewrite: %q", assessment.ImprovedPrompt)
	}
}

func TestAPIMessage(t *testing.T) {
	if got := apiMessage([]byte(`{"error":{"message":"quota exceeded"}}`)); got != "quota exceeded" {
		t.Fatalf("apiMessage() = %q", got)
	}
	if got := apiMessage([]byte(`{"error":"model not found"}`)); got != "model not found" {
		t.Fatalf("apiMessage() = %q", got)
	}
}
