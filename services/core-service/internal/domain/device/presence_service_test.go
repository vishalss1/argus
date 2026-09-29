package device

import (
	"testing"
	"time"
)

func cachedEntryFor(d *Device) resolvedDevice {
	return resolvedDevice{device: d, expires: time.Now().Add(24 * time.Hour)}
}

func TestInvalidateResolutionCacheRemovesCanonicalKey(t *testing.T) {
	// A device resolved by hardware ID is cached under both keys, mirroring
	// GetDeviceByIDOrHardwareID. Invalidating by hardware ID must drop the
	// canonical UUID key too, or a later lookup by UUID keeps resolving to the
	// stale device for the full 24h TTL.
	svc := NewPresenceService(NewService(newFakeRepository()))
	dev := &Device{ID: "11111111-2222-3333-4444-555555555555"}
	entry := cachedEntryFor(dev)

	svc.resolutionCache.Store("hw-abc123", entry)
	svc.resolutionCache.Store(dev.ID, entry)

	svc.InvalidateResolutionCache("hw-abc123")

	if _, ok := svc.resolutionCache.Load("hw-abc123"); ok {
		t.Error("expected hardware ID key to be evicted")
	}
	if _, ok := svc.resolutionCache.Load(dev.ID); ok {
		t.Error("expected canonical UUID key to be evicted, got stale cache entry")
	}
}

func TestInvalidateResolutionCacheRemovesHardwareKeyWhenGivenUUID(t *testing.T) {
	// Invalidation can also arrive with the canonical UUID (DeleteDevice does
	// exactly this). The hardware ID key must go too, otherwise a lookup by
	// hardware ID resurrects a deleted device.
	svc := NewPresenceService(NewService(newFakeRepository()))
	dev := &Device{ID: "11111111-2222-3333-4444-555555555555"}
	entry := cachedEntryFor(dev)

	svc.resolutionCache.Store("hw-abc123", entry)
	svc.resolutionCache.Store(dev.ID, entry)

	svc.InvalidateResolutionCache(dev.ID)

	if _, ok := svc.resolutionCache.Load(dev.ID); ok {
		t.Error("expected canonical UUID key to be evicted")
	}
	if _, ok := svc.resolutionCache.Load("hw-abc123"); ok {
		t.Error("expected hardware ID key to be evicted, got stale cache entry")
	}
}

func TestInvalidateResolutionCacheLeavesOtherDevicesAlone(t *testing.T) {
	// Eviction must be scoped to the one device: dropping the whole map would
	// silently turn this into a cache flush and cost a DB read per device.
	svc := NewPresenceService(NewService(newFakeRepository()))
	target := &Device{ID: "aaaaaaaa-0000-0000-0000-000000000001"}
	other := &Device{ID: "bbbbbbbb-0000-0000-0000-000000000002"}

	targetEntry := cachedEntryFor(target)
	otherEntry := cachedEntryFor(other)
	svc.resolutionCache.Store("hw-target", targetEntry)
	svc.resolutionCache.Store(target.ID, targetEntry)
	svc.resolutionCache.Store("hw-other", otherEntry)
	svc.resolutionCache.Store(other.ID, otherEntry)

	svc.InvalidateResolutionCache("hw-target")

	if _, ok := svc.resolutionCache.Load(target.ID); ok {
		t.Error("expected target device keys to be evicted")
	}
	for _, key := range []string{"hw-other", other.ID} {
		if _, ok := svc.resolutionCache.Load(key); !ok {
			t.Errorf("expected unrelated key %q to survive invalidation of another device", key)
		}
	}
}

func TestInvalidateResolutionCacheUnknownKeyIsNoop(t *testing.T) {
	// DeleteDevice invalidates a UUID that may never have been cached. That
	// must not panic, and must not clear entries for real devices.
	svc := NewPresenceService(NewService(newFakeRepository()))
	other := &Device{ID: "bbbbbbbb-0000-0000-0000-000000000002"}
	svc.resolutionCache.Store(other.ID, cachedEntryFor(other))

	svc.InvalidateResolutionCache("cccccccc-0000-0000-0000-000000000003")

	if _, ok := svc.resolutionCache.Load(other.ID); !ok {
		t.Error("expected unrelated entry to survive invalidation of an unknown key")
	}
}
