package embedding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// BenchmarkEmbedding measures the latency and throughput of the LocalProvider.
// It requires the model files to be present in /app/models/all-MiniLM-L6-v2.
func BenchmarkEmbedding(b *testing.B) {
	modelPath := os.Getenv("EMBED_MODEL_PATH")
	if modelPath == "" {
		modelPath = "/app/models/all-MiniLM-L6-v2"
	}

	// Check if model exists, skip if not (e.g. running outside docker)
	if _, err := os.Stat(filepath.Join(modelPath, "model.onnx")); os.IsNotExist(err) {
		b.Skipf("Model not found at %s. Skipping benchmark.", modelPath)
	}

	provider, err := NewLocalProvider(modelPath, 384)
	if err != nil {
		b.Fatalf("Failed to initialize LocalProvider: %v", err)
	}

	ctx := context.Background()
	text := "This is a sample telemetry log message indicating a network disconnect on eth0."

	parallelismLevels := []int{1, 4, 8, 16}

	for _, p := range parallelismLevels {
		b.Run(fmt.Sprintf("Parallelism-%d", p), func(b *testing.B) {
			b.SetParallelism(p)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := provider.Embed(ctx, text)
					if err != nil {
						b.Fatalf("Embed failed: %v", err)
					}
				}
			})
			b.StopTimer()

			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			b.ReportMetric(float64(m.Sys)/1024/1024, "Peak_RSS_MB")
			b.ReportMetric(float64(m.PauseTotalNs)/1e6, "GC_Pause_ms")

			b.ReportAllocs()
		})
	}
}
