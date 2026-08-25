package orm

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

type lazyLoaderTarget interface {
	setAnyLoader(func(context.Context) (any, error)) error
}

// Lazy 表示显式延迟加载的单值关联。
type Lazy[T any] struct {
	state *lazyState[T]
}

type lazyState[T any] struct {
	mu     sync.Mutex
	loader func(context.Context) (T, error)
	value  T
	loaded bool
}

// NewLazy 创建单值延迟加载器。
func NewLazy[T any](loader func(context.Context) (T, error)) Lazy[T] {
	return Lazy[T]{state: &lazyState[T]{loader: loader}}
}

// Load 返回延迟加载值；首次成功后会缓存结果。
func (l *Lazy[T]) Load(ctx context.Context) (T, error) {
	var zero T
	if ctx == nil {
		return zero, fmt.Errorf("goark-orm: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if l == nil {
		return zero, fmt.Errorf("goark-orm: lazy value is nil")
	}
	state := l.state
	if state == nil {
		return zero, fmt.Errorf("goark-orm: lazy loader is nil")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.loaded {
		return state.value, nil
	}
	loader := state.loader
	if loader == nil {
		return zero, fmt.Errorf("goark-orm: lazy loader is nil")
	}
	value, err := loader(ctx)
	if err != nil {
		return zero, err
	}
	state.value = value
	state.loaded = true
	state.loader = nil
	return value, nil
}

// Loaded 返回是否已经成功加载。
func (l *Lazy[T]) Loaded() bool {
	if l == nil {
		return false
	}
	state := l.state
	if state == nil {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.loaded
}

// Value 返回已加载值；未加载时 ok 为 false。
func (l *Lazy[T]) Value() (value T, ok bool) {
	if l == nil {
		return value, false
	}
	state := l.state
	if state == nil {
		return value, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.loaded {
		return value, false
	}
	return state.value, true
}

// Reset 替换延迟加载器并清空已加载值。
func (l *Lazy[T]) Reset(loader func(context.Context) (T, error)) {
	if l == nil {
		return
	}
	state := l.state
	if state == nil {
		state = &lazyState[T]{}
		l.state = state
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	var zero T
	state.value = zero
	state.loaded = false
	state.loader = loader
}

func (l *Lazy[T]) setAnyLoader(loader func(context.Context) (any, error)) error {
	if l == nil {
		return fmt.Errorf("goark-orm: lazy value is nil")
	}
	if loader == nil {
		l.Reset(nil)
		return nil
	}
	l.Reset(func(ctx context.Context) (T, error) {
		value, err := loader(ctx)
		if err != nil {
			var zero T
			return zero, err
		}
		return convertLazyValue[T](value)
	})
	return nil
}

// LazySlice 表示显式延迟加载的集合关联。
type LazySlice[T any] struct {
	lazy Lazy[[]T]
}

// NewLazySlice 创建集合延迟加载器。
func NewLazySlice[T any](loader func(context.Context) ([]T, error)) LazySlice[T] {
	return LazySlice[T]{lazy: NewLazy(loader)}
}

// Load 返回延迟加载集合；首次成功后会缓存结果。
func (l *LazySlice[T]) Load(ctx context.Context) ([]T, error) {
	if l == nil {
		return nil, fmt.Errorf("goark-orm: lazy slice is nil")
	}
	return l.lazy.Load(ctx)
}

// Loaded 返回集合是否已经成功加载。
func (l *LazySlice[T]) Loaded() bool {
	if l == nil {
		return false
	}
	return l.lazy.Loaded()
}

// Value 返回已加载集合；未加载时 ok 为 false。
func (l *LazySlice[T]) Value() (value []T, ok bool) {
	if l == nil {
		return nil, false
	}
	return l.lazy.Value()
}

// Reset 替换集合延迟加载器并清空已加载值。
func (l *LazySlice[T]) Reset(loader func(context.Context) ([]T, error)) {
	if l == nil {
		return
	}
	l.lazy.Reset(loader)
}

func (l *LazySlice[T]) setAnyLoader(loader func(context.Context) (any, error)) error {
	if l == nil {
		return fmt.Errorf("goark-orm: lazy slice is nil")
	}
	return l.lazy.setAnyLoader(loader)
}

func convertLazyValue[T any](value any) (T, error) {
	var zero T
	if typed, ok := value.(T); ok {
		return typed, nil
	}
	targetType := reflect.TypeFor[T]()
	if value == nil {
		return zero, nil
	}
	source := reflect.ValueOf(value)
	if source.Type().AssignableTo(targetType) {
		return source.Interface().(T), nil
	}
	if source.Type().ConvertibleTo(targetType) {
		return source.Convert(targetType).Interface().(T), nil
	}
	return zero, fmt.Errorf("goark-orm: lazy value %s cannot assign to %s", source.Type(), targetType)
}
