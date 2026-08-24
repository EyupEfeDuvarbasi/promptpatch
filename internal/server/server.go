// Package server exposes PromptPatch as a production HTTP service.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

const (
	defaultAddr           = "127.0.0.1:8080"
	defaultTimeout        = 60 * time.Second
	defaultMaxConcurrency = 2
)

type Config struct {
	Addr           string
	Token          string
	OllamaURL      string
	OllamaModel    string
	Timeout        time.Duration
	MaxConcurrency int
}

type Server struct {
	config Config
	mux    *http.ServeMux
	limit  chan struct{}
	client *http.Client
}

type ImproveRequest struct {
	Prompt      string   `json:"prompt"`
	Questions   []string `json:"questions,omitempty"`
	Answers     []string `json:"answers,omitempty"`
	ChatContext string   `json:"chat_context,omitempty"`
}

type ImproveResponse struct {
	OriginalScore  int               `json:"original_score"`
	ImprovedScore  int               `json:"improved_score"`
	Original       []score.Criterion `json:"original"`
	Improved       []score.Criterion `json:"improved"`
	Questions      []string          `json:"questions,omitempty"`
	ImprovedPrompt string            `json:"improved_prompt"`
	Source         string            `json:"source"`
	Warning        string            `json:"warning,omitempty"`
}

func New(config Config) *Server {
	config = normalizeConfig(config)
	server := &Server{
		config: config,
		mux:    http.NewServeMux(),
		limit:  make(chan struct{}, config.MaxConcurrency),
		client: &http.Client{Timeout: config.Timeout + 5*time.Second},
	}
	server.routes()
	return server
}

func FromEnv(getenv func(string) string) (Config, error) {
	config := Config{
		Addr:        getenv("PROMPTPATCH_SERVER_ADDR"),
		Token:       getenv("PROMPTPATCH_SERVER_TOKEN"),
		OllamaURL:   getenv("PROMPTPATCH_OLLAMA_URL"),
		OllamaModel: getenv("PROMPTPATCH_OLLAMA_MODEL"),
	}
	if timeout := strings.TrimSpace(getenv("PROMPTPATCH_TIMEOUT")); timeout != "" {
		parsed, err := time.ParseDuration(timeout)
		if err != nil {
			return Config{}, fmt.Errorf("PROMPTPATCH_TIMEOUT geçersiz: %w", err)
		}
		config.Timeout = parsed
	}
	if maxConcurrency := strings.TrimSpace(getenv("PROMPTPATCH_MAX_CONCURRENCY")); maxConcurrency != "" {
		parsed, err := strconv.Atoi(maxConcurrency)
		if err != nil || parsed < 1 {
			return Config{}, errors.New("PROMPTPATCH_MAX_CONCURRENCY pozitif tam sayı olmalı")
		}
		config.MaxConcurrency = parsed
	}
	config = normalizeConfig(config)
	if config.Token == "" && !isLoopbackAddr(config.Addr) {
		return Config{}, errors.New("PROMPTPATCH_SERVER_TOKEN public bind için zorunlu")
	}
	return config, nil
}

func (s *Server) ListenAndServe() error {
	httpServer := &http.Server{
		Addr:              s.config.Addr,
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return httpServer.ListenAndServe()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("POST /v1/improve", s.auth(s.improve))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	client, err := s.ollamaClient()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}
	if _, err := client.InstalledOllamaModel(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) improve(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, "JSON body gerekli")
		return
	}
	defer r.Body.Close()
	var request ImproveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "JSON çözümlenemedi")
		return
	}
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt gerekli")
		return
	}
	if len(request.Questions) > 2 || len(request.Answers) > 2 {
		writeError(w, http.StatusBadRequest, "en fazla iki soru ve cevap desteklenir")
		return
	}
	select {
	case s.limit <- struct{}{}:
		defer func() { <-s.limit }()
	default:
		writeError(w, http.StatusTooManyRequests, "sunucu meşgul; daha sonra tekrar deneyin")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.Timeout)
	defer cancel()
	response := s.rewrite(ctx, request)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) rewrite(ctx context.Context, request ImproveRequest) ImproveResponse {
	client, err := s.ollamaClient()
	if err == nil {
		if assessment, improveErr := client.DynamicImproveWithContext(ctx, request.Prompt, request.ChatContext, request.Questions, request.Answers); improveErr == nil {
			original := score.Evaluate(request.Prompt)
			if len(assessment.Questions) > 0 {
				if len(request.Answers) > 0 {
					err = fmt.Errorf("model cevaplardan sonra ek soru istedi")
				} else {
					return ImproveResponse{
						OriginalScore: original.Score, Original: original.Criteria,
						Questions: assessment.Questions, Source: "ollama",
					}
				}
			}
			if err != nil {
				// fall through to deterministic local fallback
			} else if strings.TrimSpace(assessment.ImprovedPrompt) == "" {
				err = fmt.Errorf("model iyileştirilmiş prompt üretmedi")
			} else {
				improved := score.Evaluate(assessment.ImprovedPrompt)
				if improved.Score <= original.Score || leaksClarifyingQuestion(assessment.ImprovedPrompt, request.Questions) {
					err = fmt.Errorf("model çıktısı skoru artırmadı veya yerel doğrulamadan geçmedi: %d -> %d", original.Score, improved.Score)
				} else {
					return ImproveResponse{
						OriginalScore: original.Score, ImprovedScore: improved.Score,
						Original: original.Criteria, Improved: improved.Criteria,
						Questions: assessment.Questions, ImprovedPrompt: assessment.ImprovedPrompt, Source: "ollama",
					}
				}
			}
		} else {
			err = improveErr
		}
		if fallback, fallbackErr := client.ImproveWithContext(ctx, request.Prompt, request.ChatContext, request.Questions, request.Answers); fallbackErr == nil {
			improved := score.Evaluate(fallback.ImprovedPrompt)
			original := score.Evaluate(request.Prompt)
			if strings.TrimSpace(fallback.ImprovedPrompt) != "" && improved.Score > original.Score && !leaksClarifyingQuestion(fallback.ImprovedPrompt, request.Questions) {
				return ImproveResponse{
					OriginalScore: original.Score, ImprovedScore: improved.Score,
					Original: original.Criteria, Improved: improved.Criteria,
					ImprovedPrompt: fallback.ImprovedPrompt, Source: "ollama",
					Warning: errString(err),
				}
			}
		}
	}
	improvedPrompt := cli.LocalImproveWithContext(request.Prompt, request.ChatContext, request.Questions, request.Answers)
	original := score.Evaluate(request.Prompt)
	improved := score.Evaluate(improvedPrompt)
	response := ImproveResponse{
		OriginalScore: original.Score, ImprovedScore: improved.Score,
		Original: original.Criteria, Improved: improved.Criteria,
		ImprovedPrompt: improvedPrompt, Source: "local",
	}
	if err != nil {
		response.Warning = err.Error()
	}
	return response
}

func leaksClarifyingQuestion(prompt string, questions []string) bool {
	normalized := strings.ToLower(prompt)
	if strings.Contains(normalized, "soru:") || strings.Contains(normalized, "cevap:") {
		return true
	}
	for _, question := range questions {
		question = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(question, "?")))
		if question != "" && strings.Contains(normalized, question) {
			return true
		}
	}
	return false
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Server) ollamaClient() (llm.Client, error) {
	client, err := llm.New(llm.Ollama, "")
	if err != nil {
		return llm.Client{}, err
	}
	client.HTTPClient = s.client
	if s.config.OllamaURL != "" {
		client.URL = strings.TrimRight(s.config.OllamaURL, "/")
	}
	if s.config.OllamaModel != "" {
		client.Model = s.config.OllamaModel
	}
	return client, nil
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.Token == "" {
			if isLoopbackAddr(s.config.Addr) {
				next(w, r)
				return
			}
			writeError(w, http.StatusUnauthorized, "server token yapılandırılmamış")
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.config.Token)) != 1 {
			writeError(w, http.StatusUnauthorized, "yetkisiz")
			return
		}
		next(w, r)
	}
}

func isLoopbackAddr(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address == "localhost"
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	return net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

func normalizeConfig(config Config) Config {
	if strings.TrimSpace(config.Addr) == "" {
		config.Addr = defaultAddr
	}
	if strings.TrimSpace(config.OllamaURL) == "" {
		config.OllamaURL = "http://127.0.0.1:11434/api/generate"
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = defaultMaxConcurrency
	}
	return config
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
