package embedding

import (
	"context"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shota3506/onnxruntime-purego/onnxruntime"
	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/ai"
)

// LocalProvider generates embeddings using a local ONNX model.
// Note: The underlying onnxruntime.Session is thread-safe for concurrent Run() calls,
// so no mutex is required for the Embed() method.
type LocalProvider struct {
	rt        *onnxruntime.Runtime
	session   *onnxruntime.Session
	tk        *tokenizer.Tokenizer
	dimension int

	isClosed  atomic.Bool
	closeOnce sync.Once
}

func NewLocalProvider(modelPath string, dimension int) (*LocalProvider, error) {
	// Initialize the runtime. The library looks in standard paths or an empty string to auto-discover
	rt, err := onnxruntime.NewRuntime("", 23) // API version 23 for ONNX Runtime 1.23.0
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime: %w", err)
	}

	env, err := rt.NewEnv("argus", onnxruntime.LoggingLevelWarning)
	if err != nil {
		return nil, fmt.Errorf("failed to create onnx env: %w", err)
	}

	onnxPath := filepath.Join(modelPath, "model.onnx")
	session, err := rt.NewSession(env, onnxPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create session from %s: %w", onnxPath, err)
	}

	tokPath := filepath.Join(modelPath, "tokenizer.json")
	tk, err := pretrained.FromFile(tokPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer from %s: %w", tokPath, err)
	}

	provider := &LocalProvider{
		rt:        rt,
		session:   session,
		tk:        tk,
		dimension: dimension,
	}

	vec, err := provider.Embed(context.Background(), "startup check")
	if err != nil {
		return nil, fmt.Errorf("startup embedding check failed: %w", err)
	}
	if len(vec) != dimension {
		return nil, fmt.Errorf("expected embedding dimension %d, got %d", dimension, len(vec))
	}
	
	// Publish model info metric
	modelBaseName := filepath.Base(modelPath)
	ai.EmbedModelInfo.WithLabelValues(modelBaseName, fmt.Sprintf("%d", dimension)).Set(1)

	log.Printf("[EMBEDDING] Local provider initialized successfully: model=%s, dimension=%d, onnx_runtime=1.23.0", modelBaseName, dimension)

	return provider, nil
}

func (p *LocalProvider) Embed(ctx context.Context, text string) (embedding []float32, err error) {
	if p.isClosed.Load() {
		return nil, fmt.Errorf("local provider is closed")
	}

	ai.EmbedRequestsTotal.Inc()
	start := time.Now()
	defer func() {
		ai.EmbedDurationSeconds.Observe(time.Since(start).Seconds())
	}()

	en, err := p.tk.EncodeSingle(text, true)
	if err != nil {
		ai.EmbedErrorsTotal.WithLabelValues("tokenizer").Inc()
		return nil, fmt.Errorf("tokenizer error: %w", err)
	}

	ids := en.Ids
	mask := en.AttentionMask
	typeIds := en.TypeIds

	int64Ids := make([]int64, len(ids))
	int64Mask := make([]int64, len(mask))
	int64TypeIds := make([]int64, len(typeIds))

	for i := range ids {
		int64Ids[i] = int64(ids[i])
		int64Mask[i] = int64(mask[i])
		int64TypeIds[i] = int64(typeIds[i])
	}

	inputShape := []int64{1, int64(len(ids))}

	inputIdsTensor, err := onnxruntime.NewTensorValue(p.rt, int64Ids, inputShape)
	if err != nil {
		ai.EmbedErrorsTotal.WithLabelValues("onnx_tensor").Inc()
		return nil, err
	}
	defer inputIdsTensor.Close()

	attentionMaskTensor, err := onnxruntime.NewTensorValue(p.rt, int64Mask, inputShape)
	if err != nil {
		ai.EmbedErrorsTotal.WithLabelValues("onnx_tensor").Inc()
		return nil, err
	}
	defer attentionMaskTensor.Close()

	tokenTypeIdsTensor, err := onnxruntime.NewTensorValue(p.rt, int64TypeIds, inputShape)
	if err != nil {
		ai.EmbedErrorsTotal.WithLabelValues("onnx_tensor").Inc()
		return nil, err
	}
	defer tokenTypeIdsTensor.Close()

	inputs := map[string]*onnxruntime.Value{
		"input_ids":      inputIdsTensor,
		"attention_mask": attentionMaskTensor,
		"token_type_ids": tokenTypeIdsTensor,
	}

	outputs, err := p.session.Run(ctx, inputs)
	if err != nil {
		ai.EmbedErrorsTotal.WithLabelValues("onnx_run").Inc()
		return nil, fmt.Errorf("onnx session run error: %w", err)
	}

	for _, out := range outputs {
		defer out.Close()
	}

	outputValue, ok := outputs["last_hidden_state"]
	if !ok {
		// Fallback to pooler_output if last_hidden_state is not present (for some models)
		outputValue, ok = outputs["pooler_output"]
		if !ok {
			ai.EmbedErrorsTotal.WithLabelValues("onnx_missing_output").Inc()
			return nil, fmt.Errorf("could not find last_hidden_state or pooler_output in outputs")
		}
	}

	data, shape, err := onnxruntime.GetTensorData[float32](outputValue)
	if err != nil {
		ai.EmbedErrorsTotal.WithLabelValues("onnx_tensor_data").Inc()
		return nil, err
	}

	// mean pooling manually
	embedding = make([]float32, p.dimension)
	validTokens := 0

	if len(shape) == 3 {
		for i := 0; i < len(ids); i++ {
			if mask[i] == 1 {
				validTokens++
				for j := 0; j < p.dimension; j++ {
					if i*p.dimension+j < len(data) {
						embedding[j] += data[i*p.dimension+j]
					}
				}
			}
		}
		if validTokens > 0 {
			for j := 0; j < p.dimension; j++ {
				embedding[j] /= float32(validTokens)
			}
		}
	} else if len(shape) == 2 {
		// pooler output
		for j := 0; j < p.dimension; j++ {
			if j < len(data) {
				embedding[j] = data[j]
			}
		}
	}

	// Normalize (L2)
	var sum float32
	for j := 0; j < p.dimension; j++ {
		sum += embedding[j] * embedding[j]
	}
	if sum > 0 {
		norm := float32(math.Sqrt(float64(sum)))
		for j := 0; j < p.dimension; j++ {
			embedding[j] /= norm
		}
	}

	return embedding, nil
}

// Close releases the underlying ONNX Runtime session and runtime resources.
func (p *LocalProvider) Close() error {
	p.closeOnce.Do(func() {
		p.isClosed.Store(true)
		if p.session != nil {
			p.session.Close()
		}
		// onnxruntime-purego does not expose a Release method for Runtime
	})
	return nil
}
