package runtime

import (
	"context"
	"fmt"
	"sync"
)

// BlockingCache 使用每个 key 一个加载权来合并并发缓存 miss。
type BlockingCache struct {
	delegate Cache
	mu       sync.Mutex
	flights  map[string]chan struct{}
}

// NewBlockingCache 创建缓存 miss 合并装饰器。
func NewBlockingCache(delegate Cache) *BlockingCache {
	return &BlockingCache{
		delegate: delegate,
		flights:  make(map[string]chan struct{}),
	}
}

// Get 读取缓存；同一 key 已有加载者时等待其 Put、Remove、Clear 或 ReleaseMiss。
func (c *BlockingCache) Get(ctx context.Context, key string) (any, bool, error) {
	if err := requireCacheContext(ctx); err != nil {
		return nil, false, err
	}
	if c == nil || c.delegate == nil {
		return nil, false, fmt.Errorf("goark-orm: blocking cache delegate is nil")
	}
	for {
		value, ok, err := c.delegate.Get(ctx, key)
		if err != nil || ok {
			return value, ok, err
		}
		wait, owner := c.beginFlight(key)
		if owner {
			return nil, false, nil
		}
		select {
		case <-wait:
			if err := requireCacheContext(ctx); err != nil {
				return nil, false, err
			}
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}
}

// Put 写入缓存并释放同 key 的等待者。
func (c *BlockingCache) Put(ctx context.Context, key string, value any) error {
	if c == nil || c.delegate == nil {
		return fmt.Errorf("goark-orm: blocking cache delegate is nil")
	}
	err := c.delegate.Put(ctx, key, value)
	c.releaseFlight(key)
	return err
}

// Remove 删除缓存并释放同 key 的等待者。
func (c *BlockingCache) Remove(ctx context.Context, key string) error {
	if c == nil || c.delegate == nil {
		return fmt.Errorf("goark-orm: blocking cache delegate is nil")
	}
	err := c.delegate.Remove(ctx, key)
	c.releaseFlight(key)
	return err
}

// Clear 清空缓存并释放全部等待者。
func (c *BlockingCache) Clear(ctx context.Context) error {
	if c == nil || c.delegate == nil {
		return fmt.Errorf("goark-orm: blocking cache delegate is nil")
	}
	err := c.delegate.Clear(ctx)
	c.releaseAllFlights()
	return err
}

// ReleaseMiss 释放一次未写入缓存的 miss 加载权。
func (c *BlockingCache) ReleaseMiss(ctx context.Context, key string) error {
	if ctx == nil {
		return fmt.Errorf("goark-orm: context is nil")
	}
	if c == nil {
		return fmt.Errorf("goark-orm: blocking cache is nil")
	}
	c.releaseFlight(key)
	return nil
}

// Stats 返回底层缓存的统计快照。
func (c *BlockingCache) Stats() CacheStats {
	if c == nil || c.delegate == nil {
		return CacheStats{}
	}
	provider, ok := c.delegate.(CacheStatsProvider)
	if !ok {
		return CacheStats{}
	}
	return provider.Stats()
}

func (c *BlockingCache) beginFlight(key string) (<-chan struct{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flights == nil {
		c.flights = make(map[string]chan struct{})
	}
	if wait, ok := c.flights[key]; ok {
		return wait, false
	}
	wait := make(chan struct{})
	c.flights[key] = wait
	return wait, true
}

func (c *BlockingCache) releaseFlight(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flights == nil {
		return
	}
	wait, ok := c.flights[key]
	if !ok {
		return
	}
	delete(c.flights, key)
	close(wait)
}

func (c *BlockingCache) releaseAllFlights() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, wait := range c.flights {
		delete(c.flights, key)
		close(wait)
	}
}
