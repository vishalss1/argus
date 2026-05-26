package embedding

import "context"

type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
