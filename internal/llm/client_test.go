package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

const apiAssessment = `{"clarity":80,"specificity":70,"context":60,"constraints":50,"purpose":90,"questions":[],"improved_prompt":"Detaylı prompt","improved_clarity":90,"improved_specificity":80,"improved_context":70,"improved_constraints":80,"improved_purpose":100}`

func TestAssessSendsProviderAuthAndParsesOutput(t *testing.T) {
	for _, provider := range []Provider{OpenAI, Gemini, Anthropic} {
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

func TestAPIMessage(t *testing.T) {
	if got := apiMessage([]byte(`{"error":{"message":"quota exceeded"}}`)); got != "quota exceeded" {
		t.Fatalf("apiMessage() = %q", got)
	}
}
