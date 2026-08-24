// Package api calls a central PromptPatch server from client devices.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

type Client struct {
	URL        string
	Token      string
	HTTPClient *http.Client
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

func (c Client) Improve(ctx context.Context, request ImproveRequest) (ImproveResponse, error) {
	if strings.TrimSpace(c.URL) == "" {
		return ImproveResponse{}, fmt.Errorf("PromptPatch server URL boş")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return ImproveResponse{}, err
	}
	url := strings.TrimRight(c.URL, "/") + "/v1/improve"
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ImproveResponse{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.Token) != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 65 * time.Second}
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return ImproveResponse{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return ImproveResponse{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ImproveResponse{}, fmt.Errorf("PromptPatch server %s: %s", response.Status, apiMessage(responseBody))
	}
	var improved ImproveResponse
	if err := json.Unmarshal(responseBody, &improved); err != nil {
		return ImproveResponse{}, fmt.Errorf("PromptPatch server yanıtı çözümlenemedi: %w", err)
	}
	if strings.TrimSpace(improved.ImprovedPrompt) == "" && len(improved.Questions) == 0 {
		return ImproveResponse{}, fmt.Errorf("PromptPatch server iyileştirilmiş prompt döndürmedi")
	}
	return improved, nil
}

func apiMessage(body []byte) string {
	var response struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &response) == nil && response.Error != "" {
		return response.Error
	}
	return strings.TrimSpace(string(body))
}
