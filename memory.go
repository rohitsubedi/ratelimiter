package ratelimiter

import (
	"runtime"
	"sync"
	"time"
)

type cacheItem struct {
	expiration time.Time
	child      *cacheItem
}

type cacheCleaner struct {
	interval *time.Timer
	stop     chan bool
}

type memoryCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	cleaner *cacheCleaner
}

func newMemoryCache(cleaningTime time.Duration) cacheInterface {
	var cleaner *cacheCleaner
	if cleaningTime > 0 {
		cleaner = &cacheCleaner{
			interval: time.NewTimer(cleaningTime),
			stop:     make(chan bool),
		}
	}

	cache := &memoryCache{
		items:   make(map[string]*cacheItem),
		cleaner: cleaner,
	}

	cache.cleanExpiredMemoryCache(cleaningTime)

	return cache
}

func (m *memoryCache) AppendEntry(key string, expirationTime time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := &cacheItem{expiration: time.Now().Add(expirationTime)}
	if v, ok := m.items[key]; ok {
		item.child = v
	}

	m.items[key] = item
	return nil
}

func (m *memoryCache) GetCount(key string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, found := m.items[key]
	if !found {
		return 0, nil
	}

	counter := 0
	for {
		if time.Now().After(item.expiration) {
			break
		}
		counter++
		item = item.child

		if item == nil {
			break
		}
	}

	return counter, nil
}

func (m *memoryCache) cleanExpiredMemoryCache(cleanerTime time.Duration) {
	if m.cleaner == nil {
		return
	}

	runtime.SetFinalizer(m.cleaner, stopCleaningRoutine)

	go func() {
		for {
			select {
			case <-m.cleaner.interval.C:
				m.unlinkExpiredCache()
				m.cleaner.interval.Reset(cleanerTime)
			case <-m.cleaner.stop:
				m.cleaner.interval.Stop()
			}
		}
	}()
}

// go routine is stopped when stop is set to true
func stopCleaningRoutine(cleaner *cacheCleaner) {
	cleaner.stop <- true
}

// delete all keys from the memory cache
func (m *memoryCache) unlinkExpiredCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, item := range m.items {
		for {
			if time.Now().After(item.expiration) {
				if item.child != nil {
					item.child = nil
				} else {
					delete(m.items, k)
				}
				break
			}

			if item.child == nil {
				break
			}
		}
	}
}
