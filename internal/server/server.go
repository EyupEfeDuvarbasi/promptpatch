// Package server exposes PromptPatch as a production HTTP service.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/cli"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

const (
	defaultAddr           = "127.0.0.1:8080"
	defaultTimeout        = 60 * time.Second
	defaultMaxConcurrency = 2
	defaultRateLimit      = 10
)

type Config struct {
	Addr           string
	Token          string
	OllamaURL      string
	OllamaModel    string
	Timeout        time.Duration
	MaxConcurrency int
	RateLimit      int
}

type Server struct {
	config Config
	mux    *http.ServeMux
	limit  chan struct{}
	client *http.Client
	rate   *rateLimiter
	stats  *metrics
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
	QualityStatus  string            `json:"quality_status,omitempty"`
	QualityMessage string            `json:"quality_message,omitempty"`
}

type rateWindow struct {
	started time.Time
	used    int
}
type rateLimiter struct {
	mu    sync.Mutex
	limit int
	// ponytail: fixed windows retain token keys; use a bounded token-bucket store if multiple rotating tokens are introduced.
	windows map[string]rateWindow
}
type metrics struct {
	mu            sync.Mutex
	requests      map[string]uint64
	fallback      uint64
	active        int
	duration      [6]uint64
	durationCount uint64
	durationNanos uint64
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{limit: limit, windows: map[string]rateWindow{}}
}
func (r *rateLimiter) allow(key string, now time.Time) (bool, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w := r.windows[key]
	if w.started.IsZero() || now.Sub(w.started) >= time.Minute {
		w = rateWindow{started: now}
	}
	if w.used >= r.limit {
		return false, max(1, int((time.Minute - now.Sub(w.started)).Seconds()))
	}
	w.used++
	r.windows[key] = w
	return true, 0
}

func newMetrics() *metrics { return &metrics{requests: map[string]uint64{}} }
func (m *metrics) record(status int, source string, fallback bool, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[strconv.Itoa(status)+"|"+source]++
	if fallback {
		m.fallback++
	}
	m.durationCount++
	m.durationNanos += uint64(elapsed)
	for i, bound := range [...]time.Duration{100 * time.Millisecond, 500 * time.Millisecond, time.Second, 5 * time.Second, 30 * time.Second, 60 * time.Second} {
		if elapsed <= bound {
			m.duration[i]++
		}
	}
}

func New(config Config) *Server {
	config = normalizeConfig(config)
	server := &Server{
		config: config,
		mux:    http.NewServeMux(),
		limit:  make(chan struct{}, config.MaxConcurrency),
		client: &http.Client{Timeout: config.Timeout + 5*time.Second},
		rate:   newRateLimiter(config.RateLimit),
		stats:  newMetrics(),
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
	if rateLimit := strings.TrimSpace(getenv("PROMPTPATCH_RATE_LIMIT_PER_MINUTE")); rateLimit != "" {
		parsed, err := strconv.Atoi(rateLimit)
		if err != nil || parsed < 1 {
			return Config{}, errors.New("PROMPTPATCH_RATE_LIMIT_PER_MINUTE pozitif tam sayı olmalı")
		}
		config.RateLimit = parsed
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
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      s.config.Timeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return httpServer.ListenAndServe()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("GET /metrics", s.prometheus)
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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "JSON çözümlenemedi")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "tek JSON nesnesi gerekli")
		return
	}
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt gerekli")
		return
	}
	if len(request.Questions) > 1 || len(request.Answers) > 1 {
		writeError(w, http.StatusBadRequest, "en fazla bir soru ve cevap desteklenir")
		return
	}
	select {
	case s.limit <- struct{}{}:
		defer func() { <-s.limit }()
	default:
		writeError(w, http.StatusTooManyRequests, "sunucu meşgul; daha sonra tekrar deneyin")
		return
	}
	s.stats.mu.Lock()
	s.stats.active++
	s.stats.mu.Unlock()
	defer func() {
		s.stats.mu.Lock()
		s.stats.active--
		s.stats.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(r.Context(), s.config.Timeout)
	defer cancel()
	started := time.Now()
	response := s.rewrite(ctx, request)
	s.stats.record(http.StatusOK, response.Source, response.Source == "local", time.Since(started))
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) rewrite(ctx context.Context, request ImproveRequest) ImproveResponse {
	original := score.Evaluate(request.Prompt)
	if len(request.Questions) == 0 && len(request.Answers) == 0 {
		if questions := cli.LocalQuestionsWithContext(original, request.Prompt, request.ChatContext); len(questions) > 0 {
			return ImproveResponse{OriginalScore: original.Score, Original: original.Criteria, Questions: questions, Source: "local"}
		}
	}
	if client, err := s.ollamaClient(); err == nil {
		rewrite, rewriteErr := client.ImproveWithContext(ctx, request.Prompt, request.ChatContext, request.Questions, request.Answers)
		if rewriteErr == nil {
			improved := score.Evaluate(rewrite.ImprovedPrompt)
			if cli.ValidRewrite(request.Prompt, rewrite.ImprovedPrompt) && !leaksClarifyingQuestion(rewrite.ImprovedPrompt, request.Questions) {
				return ImproveResponse{
					OriginalScore: original.Score, ImprovedScore: improved.Score,
					Original: original.Criteria, Improved: improved.Criteria,
					ImprovedPrompt: rewrite.ImprovedPrompt, Source: "ollama",
					QualityStatus: rewrite.QualityStatus,
				}
			}
			return ImproveResponse{OriginalScore: original.Score, Original: original.Criteria, Source: "ollama", QualityStatus: "failed", QualityMessage: "üretilen prompt kalite kontrolünden geçmedi"}
		}
		return ImproveResponse{OriginalScore: original.Score, Original: original.Criteria, Source: "ollama", QualityStatus: "failed", QualityMessage: rewriteErr.Error()}
	}
	return ImproveResponse{OriginalScore: original.Score, Original: original.Criteria, Source: "local", QualityStatus: "failed", QualityMessage: "Ollama modeli kullanılamıyor"}
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
				if ok, retry := s.rate.allow("local", time.Now()); !ok {
					s.stats.record(http.StatusTooManyRequests, "rate_limit", false, 0)
					w.Header().Set("Retry-After", strconv.Itoa(retry))
					writeError(w, http.StatusTooManyRequests, "çok fazla istek")
					return
				}
				next(w, r)
				return
			}
			writeError(w, http.StatusUnauthorized, "server token yapılandırılmamış")
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.config.Token)) != 1 {
			s.stats.record(http.StatusUnauthorized, "auth", false, 0)
			writeError(w, http.StatusUnauthorized, "yetkisiz")
			return
		}
		if ok, retry := s.rate.allow(provided, time.Now()); !ok {
			s.stats.record(http.StatusTooManyRequests, "rate_limit", false, 0)
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			writeError(w, http.StatusTooManyRequests, "çok fazla istek")
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
	if config.RateLimit <= 0 {
		config.RateLimit = defaultRateLimit
	}
	return config
}

func (s *Server) prometheus(w http.ResponseWriter, _ *http.Request) {
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for key, count := range s.stats.requests {
		parts := strings.SplitN(key, "|", 2)
		fmt.Fprintf(w, "promptpatch_requests_total{status=\"%s\",source=\"%s\"} %d\n", parts[0], parts[1], count)
	}
	fmt.Fprintf(w, "promptpatch_fallback_total %d\n", s.stats.fallback)
	fmt.Fprintf(w, "promptpatch_active_requests %d\n", s.stats.active)
	for i, bound := range []string{"0.1", "0.5", "1", "5", "30", "60"} {
		fmt.Fprintf(w, "promptpatch_request_duration_seconds_bucket{le=\"%s\"} %d\n", bound, s.stats.duration[i])
	}
	fmt.Fprintf(w, "promptpatch_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", s.stats.durationCount)
	fmt.Fprintf(w, "promptpatch_request_duration_seconds_count %d\n", s.stats.durationCount)
	fmt.Fprintf(w, "promptpatch_request_duration_seconds_sum %.6f\n", float64(s.stats.durationNanos)/float64(time.Second))
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
