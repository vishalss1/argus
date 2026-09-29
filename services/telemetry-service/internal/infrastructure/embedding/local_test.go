package embedding

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalProvider_Lifecycle(t *testing.T) {
	modelPath := os.Getenv("EMBED_MODEL_PATH")
	if modelPath == "" {
		modelPath = "/app/models/all-MiniLM-L6-v2"
	}

	if _, err := os.Stat(filepath.Join(modelPath, "model.onnx")); os.IsNotExist(err) {
		t.Skipf("Model not found at %s. Skipping lifecycle test.", modelPath)
	}

	p, err := NewLocalProvider(modelPath, 384)
	if err != nil {
		t.Fatalf("Failed to initialize LocalProvider: %v", err)
	}

	ctx := context.Background()

	// Should work normally
	_, err = p.Embed(ctx, "hello")
	if err != nil {
		t.Fatalf("Expected nil error before close, got %v", err)
	}

	// Close provider
	err = p.Close()
	if err != nil {
		t.Fatalf("Failed to close provider: %v", err)
	}

	// Idempotency check
	err = p.Close()
	if err != nil {
		t.Fatalf("Second close should not fail, got %v", err)
	}

	// Subsequent Embed should fail gracefully
	_, err = p.Embed(ctx, "hello")
	if err == nil {
		t.Fatalf("Expected error after provider is closed, got nil")
	}
	if err.Error() != "local provider is closed" {
		t.Errorf("Expected specific closed error, got %v", err)
	}
}
