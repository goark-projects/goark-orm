package runtime

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

const expressionPlanCacheMaxEntries int64 = 1024

var globalExpressionPlanCache = newExpressionPlanCache(expressionPlanCacheMaxEntries)

type expressionPlan struct {
	tokens []expressionToken
}

func compileExpressionPlan(expression string) (expressionPlan, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return expressionPlan{}, fmt.Errorf("goark-orm: dynamic SQL value expression is empty")
	}
	if plan, ok := globalExpressionPlanCache.get(expression); ok {
		return plan, nil
	}
	tokens, err := scanExpressionTokens(expression)
	if err != nil {
		return expressionPlan{}, err
	}
	plan := expressionPlan{tokens: tokens}
	globalExpressionPlanCache.store(expression, plan)
	return plan, nil
}

type expressionPlanCache struct {
	maxEntries int64
	entries    sync.Map
	size       atomic.Int64
	clearMu    sync.Mutex
}

func newExpressionPlanCache(maxEntries int64) *expressionPlanCache {
	return &expressionPlanCache{maxEntries: maxEntries}
}

func (c *expressionPlanCache) get(expression string) (expressionPlan, bool) {
	if c == nil || c.maxEntries <= 0 {
		return expressionPlan{}, false
	}
	value, ok := c.entries.Load(expression)
	if !ok {
		return expressionPlan{}, false
	}
	plan, ok := value.(expressionPlan)
	return plan, ok
}

func (c *expressionPlanCache) store(expression string, plan expressionPlan) {
	if c == nil || c.maxEntries <= 0 {
		return
	}
	if _, loaded := c.entries.LoadOrStore(expression, plan); loaded {
		return
	}
	if c.size.Add(1) <= c.maxEntries {
		return
	}
	c.clearOverflow()
}

func (c *expressionPlanCache) clearOverflow() {
	c.clearMu.Lock()
	defer c.clearMu.Unlock()
	if c.size.Load() <= c.maxEntries {
		return
	}
	c.entries.Range(func(key any, _ any) bool {
		c.entries.Delete(key)
		return true
	})
	c.size.Store(0)
}
