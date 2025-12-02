package services

import (
	"sync"
	"time"
)

type CacheItem struct {
	Data      interface{}
	ExpiresAt time.Time
}

type CacheService struct {
	cache map[string]*CacheItem
	mu    sync.RWMutex
}

func NewCacheService() *CacheService {
	cs := &CacheService{
		cache: make(map[string]*CacheItem),
	}
	// 启动定期清理过期缓存
	go cs.cleanupExpired()
	return cs
}

// Get 获取缓存
func (cs *CacheService) Get(key string) (interface{}, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	item, exists := cs.cache[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item.Data, true
}

// Set 设置缓存，ttl 为缓存时长（秒）
func (cs *CacheService) Set(key string, value interface{}, ttl int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.cache[key] = &CacheItem{
		Data:      value,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}
}

// Delete 删除缓存
func (cs *CacheService) Delete(key string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.cache, key)
}

// DeletePattern 删除匹配前缀的所有缓存
func (cs *CacheService) DeletePattern(prefix string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for key := range cs.cache {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(cs.cache, key)
		}
	}
}

// cleanupExpired 定期清理过期缓存
func (cs *CacheService) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cs.mu.Lock()
		now := time.Now()
		for key, item := range cs.cache {
			if now.After(item.ExpiresAt) {
				delete(cs.cache, key)
			}
		}
		cs.mu.Unlock()
	}
}

// 缓存键前缀常量
const (
	CacheKeyInstances = "instances:"
	CacheKeyVolumes   = "volumes:"
	CacheKeyVCNs      = "vcns:"
	CacheKeyConfig    = "config:"
	CacheKeyTenant    = "tenant:"
)
