package orm

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Cache 定义 Mapper namespace 级二级缓存 SPI。
type Cache interface {
	Get(ctx context.Context, key string) (any, bool, error)
	Put(ctx context.Context, key string, value any) error
	Remove(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// MemoryCacheOption 配置默认内存二级缓存。
type MemoryCacheOption func(*MemoryCache)

type memoryCacheEntry struct {
	key       string
	value     any
	createdAt time.Time
}

// MemoryCache 是并发安全的有界 LRU 二级缓存。
type MemoryCache struct {
	id         string
	maxEntries int
	ttl        time.Duration
	mu         sync.Mutex
	items      map[string]*list.Element
	order      *list.List
}

// NewMemoryCache 创建默认内存二级缓存。
func NewMemoryCache(id string, options ...MemoryCacheOption) *MemoryCache {
	id = strings.TrimSpace(id)
	cache := &MemoryCache{
		id:         id,
		maxEntries: 1024,
		items:      make(map[string]*list.Element),
		order:      list.New(),
	}
	for _, option := range options {
		if option != nil {
			option(cache)
		}
	}
	if cache.maxEntries < 0 {
		cache.maxEntries = 0
	}
	return cache
}

// WithMemoryCacheMaxEntries 设置最大缓存条数，0 表示不缓存任何条目。
func WithMemoryCacheMaxEntries(maxEntries int) MemoryCacheOption {
	return func(cache *MemoryCache) {
		cache.maxEntries = maxEntries
	}
}

// WithMemoryCacheTTL 设置缓存条目的存活时间。
func WithMemoryCacheTTL(ttl time.Duration) MemoryCacheOption {
	return func(cache *MemoryCache) {
		if ttl > 0 {
			cache.ttl = ttl
		}
	}
}

// Get 读取缓存条目。
func (c *MemoryCache) Get(ctx context.Context, key string) (any, bool, error) {
	if err := requireCacheContext(ctx); err != nil {
		return nil, false, err
	}
	if c == nil {
		return nil, false, fmt.Errorf("goark-orm: memory cache is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.items[key]
	if !ok {
		return nil, false, nil
	}
	entry := element.Value.(memoryCacheEntry)
	if c.ttl > 0 && time.Since(entry.createdAt) > c.ttl {
		c.removeElement(element)
		return nil, false, nil
	}
	c.order.MoveToFront(element)
	return entry.value, true, nil
}

// Put 写入缓存条目。
func (c *MemoryCache) Put(ctx context.Context, key string, value any) error {
	if err := requireCacheContext(ctx); err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("goark-orm: memory cache is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.maxEntries == 0 {
		return nil
	}
	if element, ok := c.items[key]; ok {
		element.Value = memoryCacheEntry{key: key, value: value, createdAt: time.Now()}
		c.order.MoveToFront(element)
		return nil
	}
	element := c.order.PushFront(memoryCacheEntry{key: key, value: value, createdAt: time.Now()})
	c.items[key] = element
	for c.maxEntries > 0 && c.order.Len() > c.maxEntries {
		c.removeElement(c.order.Back())
	}
	return nil
}

// Remove 删除单个缓存条目。
func (c *MemoryCache) Remove(ctx context.Context, key string) error {
	if err := requireCacheContext(ctx); err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("goark-orm: memory cache is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		c.removeElement(element)
	}
	return nil
}

// Clear 清空全部缓存条目。
func (c *MemoryCache) Clear(ctx context.Context) error {
	if err := requireCacheContext(ctx); err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("goark-orm: memory cache is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
	return nil
}

func (c *MemoryCache) removeElement(element *list.Element) {
	if c == nil || element == nil {
		return
	}
	entry := element.Value.(memoryCacheEntry)
	delete(c.items, entry.key)
	c.order.Remove(element)
}

func requireCacheContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func newMemoryCacheFromMeta(namespace string, meta CacheMeta) Cache {
	size := meta.Size
	if size <= 0 {
		size = 1024
	}
	options := []MemoryCacheOption{WithMemoryCacheMaxEntries(size)}
	if meta.FlushIntervalMillis > 0 {
		options = append(options, WithMemoryCacheTTL(time.Duration(meta.FlushIntervalMillis)*time.Millisecond))
	}
	return NewMemoryCache(namespace, options...)
}
