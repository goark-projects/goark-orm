package orm

import (
	"context"
	"testing"
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
