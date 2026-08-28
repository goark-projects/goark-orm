package runtime

import (
	"container/list"
	"database/sql"
)

// preparedStatementCache 是 Session 内部使用的预编译语句 LRU 缓存。
type preparedStatementCache struct {
	maxEntries int
	entries    map[string]*list.Element
	order      *list.List
}

// preparedStatementCacheEntry 保存最终 SQL 与其对应的预编译语句。
type preparedStatementCacheEntry struct {
	query     string
	statement *sql.Stmt
}

// newPreparedStatementCache 创建固定容量的 LRU 缓存。
func newPreparedStatementCache(maxEntries int) *preparedStatementCache {
	if maxEntries <= 0 {
		maxEntries = DefaultPreparedStatementCacheSize
	}
	return &preparedStatementCache{
		maxEntries: maxEntries,
		entries:    make(map[string]*list.Element, maxEntries),
		order:      list.New(),
	}
}

// get 返回缓存命中的预编译语句，并刷新 LRU 顺序。
func (c *preparedStatementCache) get(query string) (*sql.Stmt, bool) {
	if c == nil || c.entries == nil || c.order == nil {
		return nil, false
	}
	element := c.entries[query]
	if element == nil {
		return nil, false
	}
	c.order.MoveToFront(element)
	entry := element.Value.(preparedStatementCacheEntry)
	return entry.statement, true
}

// put 写入预编译语句，并返回需要在锁外关闭的淘汰项。
func (c *preparedStatementCache) put(query string, statement *sql.Stmt) []*sql.Stmt {
	if c == nil || statement == nil {
		return nil
	}
	if c.entries == nil {
		c.entries = make(map[string]*list.Element, c.maxEntries)
	}
	if c.order == nil {
		c.order = list.New()
	}
	if element := c.entries[query]; element != nil {
		c.order.MoveToFront(element)
		entry := element.Value.(preparedStatementCacheEntry)
		evicted := []*sql.Stmt{entry.statement}
		element.Value = preparedStatementCacheEntry{query: query, statement: statement}
		return evicted
	}
	element := c.order.PushFront(preparedStatementCacheEntry{query: query, statement: statement})
	c.entries[query] = element
	if len(c.entries) <= c.maxEntries {
		return nil
	}
	return c.evictOldest()
}

// evictOldest 移除最久未使用的语句。
func (c *preparedStatementCache) evictOldest() []*sql.Stmt {
	if c == nil || c.entries == nil || c.order == nil {
		return nil
	}
	oldest := c.order.Back()
	if oldest == nil {
		return nil
	}
	c.order.Remove(oldest)
	entry := oldest.Value.(preparedStatementCacheEntry)
	delete(c.entries, entry.query)
	return []*sql.Stmt{entry.statement}
}

// closeAll 关闭全部缓存语句并释放索引结构。
func (c *preparedStatementCache) closeAll() []error {
	if c == nil || c.entries == nil {
		return nil
	}
	errs := make([]error, 0, len(c.entries))
	for _, element := range c.entries {
		entry := element.Value.(preparedStatementCacheEntry)
		if entry.statement != nil {
			errs = append(errs, entry.statement.Close())
		}
	}
	c.entries = nil
	if c.order != nil {
		c.order.Init()
	}
	return errs
}
