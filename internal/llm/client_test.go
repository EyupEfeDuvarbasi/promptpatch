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

func TestOllamaImproveReturnsRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(`{"improved_prompt":"src/parser.go dosyasını düzelt."}`) + `}`))
	}))
	defer server.Close()
	client, err := New(Ollama, "")
	if err != nil {
		t.Fatal(err)
	}
	client.URL = server.URL
	result, err := client.Improve(context.Background(), "şunu düzelt", []string{"Hangi dosya?"}, []string{"src/parser.go"})
	if err != nil || result.ImprovedPrompt != "src/parser.go dosyasını düzelt." {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestParseOllamaRewriteAcceptsRawMarkdown(t *testing.T) {
	if got := parseOllamaRewrite("```markdown\n## Görev\nParserı düzelt.\n```"); got != "## Görev\nParserı düzelt." {
		t.Fatalf("rewrite=%q", got)
	}
	if got := parseOllamaRewrite(`{"improved_prompt":"Parserı düzelt."}`); got != "Parserı düzelt." {
		t.Fatalf("rewrite=%q", got)
	}
}

func TestRequiredFactsPreserveAnswersAndTechnicalNumbers(t *testing.T) {
	facts := requiredFacts("jetson orin nano 8 gb üzerinde 10 kamera ve 20 fps hedefle", []string{"md dosyası"})
	for _, want := range []string{"md dosyası", "jetson orin nano 8 gb", "10", "20"} {
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

func TestMissingFactsGoToRelevantSections(t *testing.T) {
	got := addMissingFacts("## Bağlam\nMevcut bağlam.\n\n## Teslimat\nPlanı sun.", []string{"Beklenen davranış veya çıktı formatı nedir?"}, []string{"md dosyası", "Jetson Orin Nano 8 GB"}, []string{"md dosyası"})
	if !strings.Contains(got, "## Teslimat\nPlanı sun.\nÇıktıyı md dosyası olarak sun.") || !strings.Contains(got, "## Bağlam\nMevcut bağlam.\n- Hedef donanım: Jetson Orin Nano 8 GB.") || strings.Contains(got, "Şu somut gereksinimi koru") {
		t.Fatalf("rewrite=%q", got)
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
	assessment, err := client.Improve(ctx, "şunu düzelt", []string{"Hangi dosya?", "Beklenen davranış ne?"}, []string{"src/parser.go", "Boş girdi hata dönsün"})
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
	if !strings.Contains(assessment.ImprovedPrompt, "## Görev") {
		t.Fatalf("structured prompt missing: %q", assessment.ImprovedPrompt)
	}
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
}
