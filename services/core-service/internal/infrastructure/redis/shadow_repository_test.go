package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/core/internal/domain/shadow"
)

func TestShadowRepository_AtomicUpdates(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	client := goredis.NewClient(&goredis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	repo := &ShadowRepository{client: client}
	ctx := context.Background()
	deviceID := "test-device-123"

	// 1. Get non-existent
	_, err = repo.Get(ctx, deviceID)
	if !errors.Is(err, shadow.ErrShadowNotFound) {
		t.Fatalf("expected ErrShadowNotFound, got %v", err)
	}

	// 2. Initial UpdateDesired
	sh1, err := repo.UpdateDesired(ctx, deviceID, json.RawMessage(`{"power":"on"}`))
	if err != nil {
		t.Fatalf("UpdateDesired failed: %v", err)
	}
	if sh1.Version != 1 {
		t.Fatalf("expected version 1, got %d", sh1.Version)
	}
	if string(sh1.Desired) != `{"power":"on"}` {
		t.Fatalf("unexpected desired: %s", string(sh1.Desired))
	}
	if string(sh1.Reported) != `{}` {
		t.Fatalf("unexpected reported: %s", string(sh1.Reported))
	}
	if !sh1.Drift {
		t.Fatalf("expected drift to be true")
	}

	// 3. UpdateReported
	sh2, err := repo.UpdateReported(ctx, deviceID, json.RawMessage(`{"power":"on"}`))
	if err != nil {
		t.Fatalf("UpdateReported failed: %v", err)
	}
	if sh2.Version != 2 {
		t.Fatalf("expected version 2, got %d", sh2.Version)
	}
	if string(sh2.Desired) != `{"power":"on"}` {
		t.Fatalf("unexpected desired: %s", string(sh2.Desired))
	}
	if string(sh2.Reported) != `{"power":"on"}` {
		t.Fatalf("unexpected reported: %s", string(sh2.Reported))
	}
	if sh2.Drift {
		t.Fatalf("expected drift to be false")
	}

	// 4. Get matches
	shGet, err := repo.Get(ctx, deviceID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if shGet.Version != 2 {
		t.Fatalf("expected version 2, got %d", shGet.Version)
	}

	// 5. Concurrent updates test
	concurrentDevID := "concurrent-device"
	var wg sync.WaitGroup
	numUpdates := 50

	for i := 0; i < numUpdates; i++ {
		wg.Add(2)
		val := i
		go func() {
			defer wg.Done()
			_, _ = repo.UpdateDesired(ctx, concurrentDevID, json.RawMessage(fmt.Sprintf(`{"step":%d}`, val)))
		}()
		go func() {
			defer wg.Done()
			_, _ = repo.UpdateReported(ctx, concurrentDevID, json.RawMessage(fmt.Sprintf(`{"step":%d}`, val)))
		}()
	}
	wg.Wait()

	finalShadow, err := repo.Get(ctx, concurrentDevID)
	if err != nil {
		t.Fatalf("Get concurrent shadow failed: %v", err)
	}
	if finalShadow.Version != int64(numUpdates*2) {
		t.Fatalf("expected version %d, got %d", numUpdates*2, finalShadow.Version)
	}
}
