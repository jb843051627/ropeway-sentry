// Package cache 提供带 TTL 的进程内快照缓存，
// 用于传感器最新读数与线路状态的热读路径，并内置过期清理协程。
package cache

import (
	"sync"
	"time"
)

// Entry 缓存条目。
type Entry struct {
	Key       string
	Value     any
	ExpiresAt time.Time
}

// Cache 并发安全的 TTL 缓存。
type Cache struct {
	mu    sync.RWMutex
	items map[string]Entry
	ttl   time.Duration
}

// New 构造缓存；ttl 为默认存活时长。
func New(ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &Cache{items: make(map[string]Entry), ttl: ttl}
}

// Set 写入条目，使用默认 TTL。
func (c *Cache) Set(key string, value any) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL 写入条目并指定 TTL。
func (c *Cache) SetWithTTL(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = Entry{Key: key, Value: value, ExpiresAt: time.Now().Add(ttl)}
}

// Get 读取条目；过期条目视为未命中并顺手删除。
func (c *Cache) Get(key string) (any, bool) {
	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(item.ExpiresAt) {
		c.Delete(key)
		return nil, false
	}
	return item.Value, true
}

// Delete 删除指定键。
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Len 当前条目数（含尚未被清理的过期项）。
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// PurgeExpired 清理全部过期条目，返回清理数量。
func (c *Cache) PurgeExpired() int {
	now := time.Now()
	purged := 0
	c.mu.Lock()
	for key, item := range c.items {
		if now.After(item.ExpiresAt) {
			delete(c.items, key)
			purged++
		}
	}
	c.mu.Unlock()
	return purged
}

// StartJanitor 启动后台清理协程，按 interval 周期回收过期条目；
// 通过 ctx 取消退出。返回停止通知通道。
func (c *Cache) StartJanitor(interval time.Duration) (stop <-chan struct{}) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			c.PurgeExpired()
		}
	}()
	return done
}
