package orm

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_whenMaxEntriesExceeded_shouldEvictLeastRecentlyUsed(t *testing.T) {
	cache := NewMemoryCache("system.user.UserMapper", WithMemoryCacheMaxEntries(2))
	ctx := context.Background()

	if err := cache.Put(ctx, "a", "A"); err != nil {
		t.Fatalf("put a failed: %v", err)
	}
	if err := cache.Put(ctx, "b", "B"); err != nil {
		t.Fatalf("put b failed: %v", err)
	}
	if _, ok, err := cache.Get(ctx, "a"); err != nil || !ok {
		t.Fatalf("expected a cache hit before eviction, ok=%v err=%v", ok, err)
	}
	if err := cache.Put(ctx, "c", "C"); err != nil {
		t.Fatalf("put c failed: %v", err)
	}

	if _, ok, err := cache.Get(ctx, "b"); err != nil || ok {
		t.Fatalf("expected b to be evicted, ok=%v err=%v", ok, err)
	}
	if value, ok, err := cache.Get(ctx, "a"); err != nil || !ok || value != "A" {
		t.Fatalf("expected a to remain cached, value=%#v ok=%v err=%v", value, ok, err)
	}
}

func TestMemoryCache_Clear_shouldDropAllEntries(t *testing.T) {
	cache := NewMemoryCache("system.user.UserMapper")
	ctx := context.Background()
	if err := cache.Put(ctx, "a", "A"); err != nil {
		t.Fatalf("put failed: %v", err)
	}

	if err := cache.Clear(ctx); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	if _, ok, err := cache.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected cache miss after clear, ok=%v err=%v", ok, err)
	}
}

func TestMemoryCache_Stats_shouldTrackProductionCounters(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	cache := NewMemoryCache(
		"system.user.UserMapper",
		WithMemoryCacheMaxEntries(2),
		WithMemoryCacheTTL(time.Second),
		withMemoryCacheClock(func() time.Time { return now }),
	)
	ctx := context.Background()

	if err := cache.Put(ctx, "a", "A"); err != nil {
		t.Fatalf("put a failed: %v", err)
	}
	if err := cache.Put(ctx, "b", "B"); err != nil {
		t.Fatalf("put b failed: %v", err)
	}
	if _, ok, err := cache.Get(ctx, "a"); err != nil || !ok {
		t.Fatalf("expected a cache hit, ok=%v err=%v", ok, err)
	}
	if _, ok, err := cache.Get(ctx, "missing"); err != nil || ok {
		t.Fatalf("expected missing cache miss, ok=%v err=%v", ok, err)
	}
	if err := cache.Put(ctx, "c", "C"); err != nil {
		t.Fatalf("put c failed: %v", err)
	}
	now = now.Add(2 * time.Second)
	if _, ok, err := cache.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected a to expire, ok=%v err=%v", ok, err)
	}
	if err := cache.Remove(ctx, "c"); err != nil {
		t.Fatalf("remove c failed: %v", err)
	}
	if err := cache.Clear(ctx); err != nil {
		t.Fatalf("clear failed: %v", err)
	}

	stats := cache.Stats()
	if stats.ID != "system.user.UserMapper" || stats.MaxEntries != 2 || stats.Len != 0 {
		t.Fatalf("unexpected cache identity stats %#v", stats)
	}
	if stats.Hits != 1 || stats.Misses != 2 || stats.Puts != 3 || stats.Removes != 1 ||
		stats.Clears != 1 || stats.Evictions != 1 || stats.Expirations != 1 {
		t.Fatalf("unexpected cache counters %#v", stats)
	}
}

func TestBlockingCache_whenConcurrentMissWaitsForPut_shouldReturnCachedValue(t *testing.T) {
	cache := NewBlockingCache(NewMemoryCache("system.user.UserMapper"))
	ctx := context.Background()

	if _, ok, err := cache.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected first caller to own cache miss, ok=%v err=%v", ok, err)
	}
	done := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		value, ok, err := cache.Get(ctx, "a")
		if err != nil {
			errs <- err
			return
		}
		if !ok {
			errs <- nil
			return
		}
		done <- value.(string)
	}()

	select {
	case value := <-done:
		t.Fatalf("waiter should block before put, got %q", value)
	case err := <-errs:
		t.Fatalf("waiter should block before put, err=%v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err := cache.Put(ctx, "a", "A"); err != nil {
		t.Fatalf("put a failed: %v", err)
	}

	select {
	case value := <-done:
		if value != "A" {
			t.Fatalf("unexpected cached value %q", value)
		}
	case err := <-errs:
		t.Fatalf("waiter failed: %v", err)
	case <-time.After(time.Second):
		t.Fatalf("waiter did not resume after put")
	}
}

func TestBlockingCache_ReleaseMiss_shouldAllowNextLoader(t *testing.T) {
	cache := NewBlockingCache(NewMemoryCache("system.user.UserMapper"))
	ctx := context.Background()

	if _, ok, err := cache.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected first caller to own cache miss, ok=%v err=%v", ok, err)
	}
	if err := cache.ReleaseMiss(ctx, "a"); err != nil {
		t.Fatalf("release miss failed: %v", err)
	}
	if _, ok, err := cache.Get(ctx, "a"); err != nil || ok {
		t.Fatalf("expected next caller to own released miss, ok=%v err=%v", ok, err)
	}
}

func TestRegistry_Cache_whenCacheRefConfigured_shouldResolveTargetNamespace(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Cache:     CacheMeta{Enabled: true, Size: 16},
		Statements: []StatementMeta{{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			SQL:       "select id from sys_user where id = #{id}",
		}},
	}); err != nil {
		t.Fatalf("register user mapper failed: %v", err)
	}
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "ProfileMapper",
		Namespace: "system.profile.ProfileMapper",
		Cache:     CacheMeta{Enabled: true, RefNamespace: "system.user.UserMapper"},
		Statements: []StatementMeta{{
			ID:        "FindByUserID",
			Namespace: "system.profile.ProfileMapper",
			FullName:  "system.profile.ProfileMapper.FindByUserID",
			Command:   StatementCommandSelect,
			SQL:       "select profile from sys_profile where user_id = #{id}",
		}},
	}); err != nil {
		t.Fatalf("register profile mapper failed: %v", err)
	}

	cache, namespace, ok := registry.Cache("system.profile.ProfileMapper")
	if !ok || cache == nil || namespace != "system.user.UserMapper" {
		t.Fatalf("expected cache-ref to resolve target namespace, namespace=%q ok=%v cache=%#v", namespace, ok, cache)
	}
}

func TestRegistry_Cache_whenBlockingConfigured_shouldUseBlockingCache(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterMapper(MapperMeta{
		TypeName:  "UserMapper",
		Namespace: "system.user.UserMapper",
		Cache:     CacheMeta{Enabled: true, Size: 16, Blocking: true},
		Statements: []StatementMeta{{
			ID:        "FindByID",
			Namespace: "system.user.UserMapper",
			FullName:  "system.user.UserMapper.FindByID",
			Command:   StatementCommandSelect,
			SQL:       "select id from sys_user where id = #{id}",
		}},
	}); err != nil {
		t.Fatalf("register mapper failed: %v", err)
	}

	cache, _, ok := registry.Cache("system.user.UserMapper")
	if !ok {
		t.Fatalf("expected cache to be registered")
	}
	if _, ok := cache.(*BlockingCache); !ok {
		t.Fatalf("expected blocking cache, got %#v", cache)
	}
	if _, ok := cache.(CacheStatsProvider); !ok {
		t.Fatalf("expected blocking cache to expose delegated stats")
	}
}
