package runtime

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

type localCache struct {
	mu     sync.RWMutex
	values map[string]reflect.Value
}

func newLocalCache() *localCache {
	return &localCache{values: make(map[string]reflect.Value)}
}

func (c *localCache) get(key string, dest any) (bool, error) {
	if c == nil {
		return false, nil
	}
	c.mu.RLock()
	value, ok := c.values[key]
	c.mu.RUnlock()
	if !ok {
		return false, nil
	}
	if err := assignCachedValue(dest, value, "local"); err != nil {
		return false, err
	}
	return true, nil
}

func (c *localCache) put(key string, dest any) error {
	if c == nil {
		return nil
	}
	value, err := cloneDestinationValue(dest)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
	return nil
}

func (c *localCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = make(map[string]reflect.Value)
}

func localCacheKey(statement StatementMeta, compiled CompiledSQL) string {
	var builder strings.Builder
	builder.Grow(len(statement.FullName) + len(compiled.CacheKey) + len(compiled.SQL) + len(compiled.Args)*24 + 2)
	builder.WriteString(statement.FullName)
	builder.WriteByte(0)
	builder.WriteString(compiled.CacheKey)
	builder.WriteByte(0)
	builder.WriteString(compiled.SQL)
	for index, arg := range compiled.Args {
		builder.WriteByte(0)
		_, _ = fmt.Fprintf(&builder, "%d:%T:%#v", index, arg, arg)
	}
	return builder.String()
}

func (s *SQLSession) queryCacheKey(statement StatementMeta, compiled CompiledSQL) (string, bool) {
	if !s.queryCacheEnabled(statement) {
		return "", false
	}
	return localCacheKey(statement, compiled), true
}

func (s *SQLSession) queryCacheEnabled(statement StatementMeta) bool {
	if s == nil {
		return false
	}
	if !shouldUseQueryCache(statement) {
		return false
	}
	if s.localCache != nil {
		return true
	}
	return s.hasSecondLevelCache(statement)
}

func cloneDestinationValue(dest any) (reflect.Value, error) {
	target, err := destination(dest)
	if err != nil {
		return reflect.Value{}, err
	}
	return cloneReflectValue(target), nil
}

func assignCachedValue(dest any, cached any, source string) error {
	target, err := destination(dest)
	if err != nil {
		return err
	}
	cloned := cachedValue(cached, target.Type())
	if !cloned.IsValid() {
		target.Set(reflect.Zero(target.Type()))
		return nil
	}
	if cloned.Type().AssignableTo(target.Type()) {
		target.Set(cloned)
		return nil
	}
	if cloned.Type().ConvertibleTo(target.Type()) {
		target.Set(cloned.Convert(target.Type()))
		return nil
	}
	return fmt.Errorf("goark-orm: %s cache value %s cannot assign to %s", source, cloned.Type(), target.Type())
}

func cachedValue(cached any, targetType reflect.Type) reflect.Value {
	if value, ok := cached.(reflect.Value); ok {
		return cloneReflectValue(value)
	}
	if cached == nil {
		return reflect.Zero(targetType)
	}
	return cloneReflectValue(reflect.ValueOf(cached))
}

func cloneReflectValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return reflect.Value{}
	}
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clonedElem := cloneReflectValue(value.Elem())
		cloned := reflect.New(value.Type().Elem())
		if clonedElem.IsValid() && clonedElem.Type().AssignableTo(value.Type().Elem()) {
			cloned.Elem().Set(clonedElem)
		} else if clonedElem.IsValid() && clonedElem.Type().ConvertibleTo(value.Type().Elem()) {
			cloned.Elem().Set(clonedElem.Convert(value.Type().Elem()))
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			item := cloneReflectValue(value.Index(index))
			if item.IsValid() && item.Type().AssignableTo(value.Type().Elem()) {
				cloned.Index(index).Set(item)
			} else if item.IsValid() && item.Type().ConvertibleTo(value.Type().Elem()) {
				cloned.Index(index).Set(item.Convert(value.Type().Elem()))
			}
		}
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			cloned.SetMapIndex(cloneReflectValue(iter.Key()), cloneReflectValue(iter.Value()))
		}
		return cloned
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clonedElem := cloneReflectValue(value.Elem())
		cloned := reflect.New(value.Type()).Elem()
		if clonedElem.IsValid() && clonedElem.Type().AssignableTo(value.Type()) {
			cloned.Set(clonedElem)
		} else {
			cloned.Set(value)
		}
		return cloned
	default:
		cloned := reflect.New(value.Type()).Elem()
		cloned.Set(value)
		return cloned
	}
}
