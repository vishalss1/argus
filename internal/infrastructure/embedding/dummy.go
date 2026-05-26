package embedding

import (
	"context"
)

type DummyProvider struct {
	Dimensions int
}

func NewDummyProvider(dimensions int) *DummyProvider {
	return &DummyProvider{Dimensions: dimensions}
}

func (p *DummyProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Return a deterministic dummy vector for the given text
	vec := make([]float32, p.Dimensions)
	for i := 0; i < p.Dimensions; i++ {
		vec[i] = float32(len(text)) / float32(i+1)
	}
	return vec, nil
}
