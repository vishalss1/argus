package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "qwen2.5:7b-instruct"
	}
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Format   string          `json:"format,omitempty"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

func (p *OllamaProvider) Reason(ctx context.Context, systemPrompt, userPrompt string) (*ReasoningResponse, error) {
	LLMRequestsTotal.WithLabelValues("ollama").Inc()
	start := time.Now()
	defer func() {
		ReasoningLatencySeconds.WithLabelValues("ollama").Observe(time.Since(start).Seconds())
	}()

	reqBody := ollamaChatRequest{
		Model: p.model,
		Messages: []ollamaMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Format: "json",
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		LLMFailuresTotal.WithLabelValues("ollama").Inc()
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
	if err != nil {
		LLMFailuresTotal.WithLabelValues("ollama").Inc()
		return nil, fmt.Errorf("create ollama request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		LLMFailuresTotal.WithLabelValues("ollama").Inc()
		return nil, fmt.Errorf("do ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		LLMFailuresTotal.WithLabelValues("ollama").Inc()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		LLMFailuresTotal.WithLabelValues("ollama").Inc()
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	var reasoningResp ReasoningResponse
	if err := json.Unmarshal([]byte(ollamaResp.Message.Content), &reasoningResp); err != nil {
		LLMFailuresTotal.WithLabelValues("ollama").Inc()
		return nil, fmt.Errorf("unmarshal reasoning response: %w", err)
	}

	return &reasoningResp, nil
}
