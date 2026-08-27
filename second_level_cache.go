package orm

import (
	"context"
	"errors"
	"reflect"
	"sort"
)

func withTransactionalSecondLevelCache() SQLSessionOption {
	return func(session *SQLSession) error {
		session.transactionalCache = true
		session.pendingSecondLevelCacheFlush = make(map[string]struct{})
		session.pendingSecondLevelCachePuts = make(map[string]map[string]reflect.Value)
		return nil
	}
}

func shouldUseSecondLevelCache(statement StatementMeta) bool {
	if !shouldUseQueryCache(statement) {
		return false
	}
	return statement.UseCache != StatementCacheDisabled
}

func shouldFlushStatementCache(statement StatementMeta) bool {
	switch statement.FlushCache {
	case StatementCacheEnabled:
		return true
	case StatementCacheDisabled:
		return false
	default:
		return statement.Command != StatementCommandSelect || statement.AffectData
	}
}

func shouldUseQueryCache(statement StatementMeta) bool {
	return statement.Command == StatementCommandSelect &&
		!statement.AffectData &&
		statement.UseCache != StatementCacheDisabled
}

func (s *SQLSession) hasSecondLevelCache(statement StatementMeta) bool {
	if s == nil || s.registry == nil || !s.cacheEnabled || !shouldUseSecondLevelCache(statement) {
		return false
	}
	_, _, ok := s.registry.Cache(statement.Namespace)
	return ok
}

func (s *SQLSession) getSecondLevelCache(ctx context.Context, statement StatementMeta, key string, dest any) (bool, error) {
	if s == nil || s.registry == nil || !s.cacheEnabled || !shouldUseSecondLevelCache(statement) {
		return false, nil
	}
	cache, namespace, ok := s.registry.Cache(statement.Namespace)
	if !ok {
		return false, nil
	}
	if s.isSecondLevelCacheFlushPending(namespace) {
		return false, nil
	}
	value, ok, err := cache.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := assignCachedValue(dest, value, "second-level"); err != nil {
		return false, err
	}
	if err := s.putLocalCache(key, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLSession) putSecondLevelCache(ctx context.Context, statement StatementMeta, key string, dest any) error {
	if s == nil || s.registry == nil || !s.cacheEnabled || !shouldUseSecondLevelCache(statement) {
		return nil
	}
	cache, namespace, ok := s.registry.Cache(statement.Namespace)
	if !ok {
		return nil
	}
	value, err := cloneDestinationValue(dest)
	if err != nil {
		return err
	}
	if s.transactionalCache {
		s.putPendingSecondLevelCache(namespace, key, value)
		return nil
	}
	return cache.Put(ctx, key, value)
}

func (s *SQLSession) releaseSecondLevelCacheMiss(ctx context.Context, statement StatementMeta, key string) {
	if s == nil || s.registry == nil || !s.cacheEnabled || !shouldUseSecondLevelCache(statement) {
		return
	}
	cache, _, ok := s.registry.Cache(statement.Namespace)
	if !ok {
		return
	}
	releaser, ok := cache.(CacheMissReleaser)
	if !ok {
		return
	}
	_ = releaser.ReleaseMiss(ctx, key)
}

func (s *SQLSession) flushStatementCaches(ctx context.Context, statement StatementMeta) error {
	if !shouldFlushStatementCache(statement) {
		return nil
	}
	s.clearLocalCache()
	return s.flushSecondLevelCache(ctx, statement.Namespace)
}

func (s *SQLSession) flushSecondLevelCache(ctx context.Context, namespace string) error {
	if s == nil || s.registry == nil || !s.cacheEnabled {
		return nil
	}
	cache, resolved, ok := s.registry.Cache(namespace)
	if !ok {
		return nil
	}
	if s.transactionalCache {
		s.markPendingSecondLevelCacheFlush(resolved)
		return nil
	}
	return cache.Clear(ctx)
}

func (s *SQLSession) putPendingSecondLevelCache(namespace string, key string, value reflect.Value) {
	s.secondLevelCacheMu.Lock()
	defer s.secondLevelCacheMu.Unlock()
	if s.pendingSecondLevelCachePuts == nil {
		s.pendingSecondLevelCachePuts = make(map[string]map[string]reflect.Value)
	}
	entries := s.pendingSecondLevelCachePuts[namespace]
	if entries == nil {
		entries = make(map[string]reflect.Value)
		s.pendingSecondLevelCachePuts[namespace] = entries
	}
	entries[key] = cloneReflectValue(value)
}

func (s *SQLSession) markPendingSecondLevelCacheFlush(namespace string) {
	s.secondLevelCacheMu.Lock()
	defer s.secondLevelCacheMu.Unlock()
	if s.pendingSecondLevelCacheFlush == nil {
		s.pendingSecondLevelCacheFlush = make(map[string]struct{})
	}
	s.pendingSecondLevelCacheFlush[namespace] = struct{}{}
	delete(s.pendingSecondLevelCachePuts, namespace)
}

func (s *SQLSession) isSecondLevelCacheFlushPending(namespace string) bool {
	if s == nil || !s.transactionalCache {
		return false
	}
	s.secondLevelCacheMu.Lock()
	defer s.secondLevelCacheMu.Unlock()
	_, ok := s.pendingSecondLevelCacheFlush[namespace]
	return ok
}

func (s *SQLSession) commitSecondLevelCache(ctx context.Context) error {
	if s == nil || s.registry == nil {
		return nil
	}
	flushes, puts := s.takePendingSecondLevelCacheChanges()
	var joined error
	for _, namespace := range flushes {
		cache, _, ok := s.registry.Cache(namespace)
		if !ok {
			continue
		}
		joined = errors.Join(joined, cache.Clear(ctx))
	}
	putNamespaces := make([]string, 0, len(puts))
	for namespace := range puts {
		putNamespaces = append(putNamespaces, namespace)
	}
	sort.Strings(putNamespaces)
	for _, namespace := range putNamespaces {
		cache, _, ok := s.registry.Cache(namespace)
		if !ok {
			continue
		}
		keys := make([]string, 0, len(puts[namespace]))
		for key := range puts[namespace] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			joined = errors.Join(joined, cache.Put(ctx, key, cloneReflectValue(puts[namespace][key])))
		}
	}
	return joined
}

func (s *SQLSession) discardSecondLevelCacheChanges() {
	if s == nil {
		return
	}
	s.secondLevelCacheMu.Lock()
	defer s.secondLevelCacheMu.Unlock()
	s.pendingSecondLevelCacheFlush = make(map[string]struct{})
	s.pendingSecondLevelCachePuts = make(map[string]map[string]reflect.Value)
}

func (s *SQLSession) takePendingSecondLevelCacheChanges() ([]string, map[string]map[string]reflect.Value) {
	s.secondLevelCacheMu.Lock()
	defer s.secondLevelCacheMu.Unlock()
	flushes := make([]string, 0, len(s.pendingSecondLevelCacheFlush))
	for namespace := range s.pendingSecondLevelCacheFlush {
		flushes = append(flushes, namespace)
	}
	sort.Strings(flushes)
	puts := make(map[string]map[string]reflect.Value, len(s.pendingSecondLevelCachePuts))
	for namespace, entries := range s.pendingSecondLevelCachePuts {
		copied := make(map[string]reflect.Value, len(entries))
		for key, value := range entries {
			copied[key] = cloneReflectValue(value)
		}
		puts[namespace] = copied
	}
	s.pendingSecondLevelCacheFlush = make(map[string]struct{})
	s.pendingSecondLevelCachePuts = make(map[string]map[string]reflect.Value)
	return flushes, puts
}
