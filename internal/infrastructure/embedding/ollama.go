package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vishalss1/argus/internal/infrastructure/ai"
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
		model = "nomic-embed-text"
	}
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

type ollamaEmbeddingRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
}

type ollamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()
	defer func() {
		ai.EmbeddingLatencySeconds.WithLabelValues("ollama").Observe(time.Since(start).Seconds())
	}()

	reqBody := ollamaEmbeddingRequest{
		Model:  p.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/embeddings", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create ollama request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, string(body))
	}

	var embResp ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	if len(embResp.Embedding) == 0 {
		return nil, fmt.Errorf("no embedding in ollama response")
	}

	return embResp.Embedding, nil
}
