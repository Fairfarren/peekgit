package cache

import (
	"testing"
	"time"
)

func TestTTLCacheGetSet(t *testing.T) {
	c := NewTTLCache[int](time.Minute)
	now := time.Now()

	c.Set("k", 42, now)
	v, ok := c.Get("k", now.Add(30*time.Second))
	if !ok || v != 42 {
		t.Fatalf("got (%d, %v)", v, ok)
	}
}

func TestTTLCacheExpiry(t *testing.T) {
	c := NewTTLCache[int](time.Second)
	now := time.Now()
	c.Set("k", 1, now)

	_, ok := c.Get("k", now.Add(2*time.Second))
	if ok {
		t.Fatalf("expected expired")
	}
}
