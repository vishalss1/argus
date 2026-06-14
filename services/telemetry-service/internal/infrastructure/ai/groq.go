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

type GroqProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewGroqProvider(apiKey, model, baseURL string) *GroqProvider {
	return &GroqProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
	}
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model          string        `json:"model"`
	Messages       []groqMessage `json:"messages"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (p *GroqProvider) Reason(ctx context.Context, systemPrompt, userPrompt string) (*ReasoningResponse, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY is not set")
	}

	LLMRequestsTotal.WithLabelValues("groq").Inc()
	start := time.Now()
	defer func() {
		ReasoningLatencySeconds.WithLabelValues("groq").Observe(time.Since(start).Seconds())
	}()

	reqBody := groqRequest{
		Model: p.model,
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		LLMFailuresTotal.WithLabelValues("groq").Inc()
		return nil, fmt.Errorf("marshal groq request: %w", err)
	}

	var resp *http.Response
	var doErr error
	delay := 1 * time.Second

	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
		if err != nil {
			LLMFailuresTotal.WithLabelValues("groq").Inc()
			return nil, fmt.Errorf("create groq request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, doErr = p.client.Do(req)
		if doErr != nil {
			LLMFailuresTotal.WithLabelValues("groq").Inc()
			return nil, fmt.Errorf("do groq request: %w", doErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			select {
			case <-ctx.Done():
				LLMFailuresTotal.WithLabelValues("groq").Inc()
				return nil, ctx.Err()
			case <-time.After(delay):
				delay *= 2
				continue
			}
		}
		break
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		LLMFailuresTotal.WithLabelValues("groq").Inc()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq API error (%d): %s", resp.StatusCode, string(body))
	}

	var groqResp groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		LLMFailuresTotal.WithLabelValues("groq").Inc()
		return nil, fmt.Errorf("decode groq response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		LLMFailuresTotal.WithLabelValues("groq").Inc()
		return nil, fmt.Errorf("no choices in groq response")
	}

	var reasoningResp ReasoningResponse
	if err := json.Unmarshal([]byte(groqResp.Choices[0].Message.Content), &reasoningResp); err != nil {
		LLMFailuresTotal.WithLabelValues("groq").Inc()
		return nil, fmt.Errorf("unmarshal reasoning response: %w", err)
	}

	return &reasoningResp, nil
}
