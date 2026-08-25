package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestImproveRequiresBearerTokenWhenConfigured(t *testing.T) {
	server := New(Config{Token: "secret", Timeout: time.Second})
	request := httptest.NewRequest(http.MethodPost, "/v1/improve", strings.NewReader(`{"prompt":"şunu düzelt"}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestImproveFallsBackToLocalRewriteWhenOllamaFails(t *testing.T) {
	server := New(Config{OllamaURL: "http://127.0.0.1:1/api/generate", Timeout: time.Millisecond})
	request := httptest.NewRequest(http.MethodPost, "/v1/improve", strings.NewReader(`{"prompt":"şunu düzelt","answers":["src/parser.go"]}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got ImproveResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "local" || got.OriginalScore == 0 || got.ImprovedScore == 0 || !strings.Contains(got.ImprovedPrompt, "şunu düzelt") {
		t.Fatalf("response=%#v", got)
	}
}

func TestImproveUsesOllamaRewrite(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(`{"original_score":30,"improved_score":80,"original_criteria":[{"Name":"Görev bağlamı","Score":30}],"improved_criteria":[{"Name":"Görev bağlamı","Score":80}],"questions":[],"improved_prompt":"src/parser.go dosyasındaki boş girdi hatasını düzelt ve JSON hata açıklaması döndür."}`) + `}`))
	}))
	defer ollama.Close()
	server := New(Config{OllamaURL: ollama.URL + "/api/generate", OllamaModel: "test-model", Timeout: time.Second})
	body := bytes.NewBufferString(`{"prompt":"şunu düzelt","questions":["Hangi dosya?"],"answers":["src/parser.go"]}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/improve", body)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got ImproveResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "ollama" || !strings.Contains(got.ImprovedPrompt, "src/parser.go") {
		t.Fatalf("response=%#v", got)
	}
}

func TestImproveFallsBackWhenOllamaLowersScore(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(`{"original_score":80,"improved_score":30,"original_criteria":[{"Name":"Kapsam","Score":80}],"improved_criteria":[{"Name":"Kapsam","Score":30}],"questions":[],"improved_prompt":"README güncelle."}`) + `}`))
	}))
	defer ollama.Close()
	server := New(Config{OllamaURL: ollama.URL + "/api/generate", OllamaModel: "test-model", Timeout: time.Second})
	body := bytes.NewBufferString(`{"prompt":"README dosyasına GitHubdan kurulumu, systemd servisini, Ollama portunun internete açılmaması gerektiğini ve /readyz kontrolünü Türkçe ekle.","questions":["Hedef depo bu README mi?"],"answers":["Evet."]}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/improve", body)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got ImproveResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "local" {
		t.Fatalf("response=%#v", got)
	}
}

func TestImproveFallsBackWhenOllamaKeepsSameScore(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(`{"original_score":40,"improved_score":40,"original_criteria":[{"Name":"Kapsam","Score":40}],"improved_criteria":[{"Name":"Kapsam","Score":40}],"questions":[],"improved_prompt":"İçerik kapsamı: README dosyasına GitHubdan kurulumu, systemd servisini, Ollama portunun internete açılmaması gerektiğini ve /readyz kontrolünü Türkçe ekle. Değişikliklerin doğruluğunu doğrula."}`) + `}`))
	}))
	defer ollama.Close()
	server := New(Config{OllamaURL: ollama.URL + "/api/generate", OllamaModel: "test-model", Timeout: time.Second})
	body := bytes.NewBufferString(`{"prompt":"README dosyasına GitHubdan kurulumu, systemd servisini, Ollama portunun internete açılmaması gerektiğini ve /readyz kontrolünü Türkçe ekle.","questions":["Hedef depo bu README mi?"]}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/improve", body)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got ImproveResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "ollama" {
		t.Fatalf("response=%#v", got)
	}
}

func TestImproveReturnsLocalQuestionsBeforeOllama(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Ollama eksik prompt için çağrılmamalı")
	}))
	defer ollama.Close()
	server := New(Config{OllamaURL: ollama.URL + "/api/generate", OllamaModel: "test-model", Timeout: time.Second})
	request := httptest.NewRequest(http.MethodPost, "/v1/improve", strings.NewReader(`{"prompt":"şunu düzelt"}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got ImproveResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "local" || len(got.Questions) == 0 || got.ImprovedPrompt != "" {
		t.Fatalf("response=%#v", got)
	}
}

func TestImproveFallsBackWhenModelAsksAgainAfterAnswers(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":` + strconv.Quote(`{"original_score":20,"improved_score":0,"original_criteria":[{"Name":"Eksik hedef","Score":20}],"improved_criteria":[],"questions":["Bir soru daha?"],"improved_prompt":""}`) + `}`))
	}))
	defer ollama.Close()
	server := New(Config{OllamaURL: ollama.URL + "/api/generate", OllamaModel: "test-model", Timeout: time.Second})
	request := httptest.NewRequest(http.MethodPost, "/v1/improve", strings.NewReader(`{"prompt":"şunu düzelt","questions":["Hangi dosya?"],"answers":["src/parser.go"]}`))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got ImproveResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "local" {
		t.Fatalf("response=%#v", got)
	}
}

func TestLeaksClarifyingQuestion(t *testing.T) {
	questions := []string{"Hata mesajının içeriği için belirli bir şablon var mı?"}
	if !leaksClarifyingQuestion("Hata mesajının içeriği için belirli bir şablon var mı? JSON kullan.", questions) {
		t.Fatal("expected question leak")
	}
	if leaksClarifyingQuestion("JSON formatı kullanılsın?", nil) {
		t.Fatal("ordinary question mark should be allowed")
	}
	if leaksClarifyingQuestion("JSON formatı kullanılsın ve mevcut davranış korunsun.", questions) {
		t.Fatal("did not expect question leak")
	}
}

func TestFromEnvParsesProductionSettings(t *testing.T) {
	values := map[string]string{
		"PROMPTPATCH_SERVER_ADDR":           "0.0.0.0:9090",
		"PROMPTPATCH_SERVER_TOKEN":          "secret",
		"PROMPTPATCH_OLLAMA_URL":            "http://ollama:11434/api/generate",
		"PROMPTPATCH_OLLAMA_MODEL":          "gemma3:4b",
		"PROMPTPATCH_TIMEOUT":               "45s",
		"PROMPTPATCH_MAX_CONCURRENCY":       "4",
		"PROMPTPATCH_RATE_LIMIT_PER_MINUTE": "7",
	}
	config, err := FromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if config.Addr != "0.0.0.0:9090" || config.Token != "secret" || config.Timeout != 45*time.Second || config.MaxConcurrency != 4 || config.RateLimit != 7 {
		t.Fatalf("config=%#v", config)
	}
}

func TestFromEnvRejectsPublicServerWithoutToken(t *testing.T) {
	values := map[string]string{"PROMPTPATCH_SERVER_ADDR": "0.0.0.0:8080"}
	if _, err := FromEnv(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected public server without token to be rejected")
	}
}

func TestImproveRejectsUnknownAndMultipleJSON(t *testing.T) {
	server := New(Config{})
	for _, body := range []string{`{"prompt":"şunu düzelt","extra":true}`, `{"prompt":"şunu düzelt"}{"prompt":"ikinci"}`} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/improve", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d", body, response.Code)
		}
	}
}

func TestRateLimitAndMetrics(t *testing.T) {
	server := New(Config{RateLimit: 1, OllamaURL: "http://127.0.0.1:1/api/generate", Timeout: time.Millisecond})
	first := httptest.NewRecorder()
	server.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/v1/improve", strings.NewReader(`{"prompt":"şunu düzelt"}`)))
	second := httptest.NewRecorder()
	server.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/v1/improve", strings.NewReader(`{"prompt":"şunu düzelt"}`)))
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") == "" {
		t.Fatalf("status=%d headers=%v", second.Code, second.Header())
	}
	metrics := httptest.NewRecorder()
	server.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Body.String(), "promptpatch_requests_total") || strings.Contains(metrics.Body.String(), "şunu düzelt") {
		t.Fatalf("metrics=%s", metrics.Body.String())
	}
}
