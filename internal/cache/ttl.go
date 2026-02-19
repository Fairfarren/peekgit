package cache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

type TTLCache[T any] struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]entry[T]
}

func NewTTLCache[T any](ttl time.Duration) *TTLCache[T] {
	return &TTLCache[T]{
		ttl:   ttl,
		items: make(map[string]entry[T]),
	}
}

func (c *TTLCache[T]) Get(key string, now time.Time) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || now.After(item.expiresAt) {
		var zero T
		return zero, false
	}
	return item.value, true
}

func (c *TTLCache[T]) Set(key string, value T, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = entry[T]{
		value:     value,
		expiresAt: now.Add(c.ttl),
	}
}
