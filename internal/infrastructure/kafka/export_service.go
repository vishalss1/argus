package kafka

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	segmentio "github.com/segmentio/kafka-go"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type ExportService struct {
	brokers   []string
	topic     string
	exportDir string
	baseURL   string
}

func NewExportService(brokers []string, topic string, exportDir string, baseURL string) (*ExportService, error) {
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return nil, fmt.Errorf("create export dir: %w", err)
	}

	return &ExportService{
		brokers:   brokers,
		topic:     topic,
		exportDir: exportDir,
		baseURL:   baseURL,
	}, nil
}

func (s *ExportService) Export(ctx context.Context, req telemetry.ExportRequest) (*telemetry.ExportResponse, error) {
	reader := segmentio.NewReader(segmentio.ReaderConfig{
		Brokers: s.brokers,
		Topic:   s.topic,
	})
	defer reader.Close()

	// 1. Seek to start time
	if err := reader.SetOffsetAt(ctx, req.From); err != nil {
		return nil, fmt.Errorf("seek to start time: %w", err)
	}

	// 2. Prepare file
	fileID := uuid.New().String()
	fileName := fmt.Sprintf("export_%s_%s.%s", req.DeviceID, fileID, req.Format)
	filePath := filepath.Join(s.exportDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create export file: %w", err)
	}
	defer file.Close()

	if req.Format == telemetry.ExportFormatCSV {
		if err := s.writeCSV(ctx, reader, file, req); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unsupported format: %s", req.Format)
	}

	// 3. Generate response
	expiresAt := time.Now().Add(24 * time.Hour)
	
	// Auto-delete after 24 hours
	go func() {
		time.Sleep(24 * time.Hour)
		_ = os.Remove(filePath)
	}()

	return &telemetry.ExportResponse{
		DownloadURL: fmt.Sprintf("%s/api/telemetry/exports/%s", s.baseURL, fileName),
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *ExportService) writeCSV(ctx context.Context, reader *segmentio.Reader, file *os.File, req telemetry.ExportRequest) error {
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write Header
	writer.Write([]string{"recorded_at", "metrics"})

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			break
		}

		if m.Time.After(req.To) {
			break
		}

		var t telemetry.Telemetry
		if err := json.Unmarshal(m.Value, &t); err != nil {
			continue
		}

		if t.DeviceID != req.DeviceID {
			continue
		}

		writer.Write([]string{
			t.RecordedAt.Format(time.RFC3339),
			string(t.Metrics),
		})
	}

	return nil
}
