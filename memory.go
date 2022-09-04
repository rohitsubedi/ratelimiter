package ratelimiter

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

type cacheItem struct {
	creationTime   time.Time
	expiryDuration time.Duration
	child          *cacheItem
}

type cacheCleaner struct {
	interval *time.Timer
	stop     chan bool
}

type memoryCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	cleaner *cacheCleaner
	config  ConfigReaderInterface
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

func (m *memoryCache) appendEntry(key string, expiryDuration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := &cacheItem{creationTime: time.Now(), expiryDuration: expiryDuration}
	if v, ok := m.items[key]; ok {
		item.child = v
	}

	m.items[key] = item
	return nil
}

func (m *memoryCache) getCount(key string, expirationDuration time.Duration) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, found := m.items[key]
	if !found {
		return 0
	}

	counter := 0
	for {
		if time.Now().After(item.creationTime.Add(expirationDuration)) {
			break
		}
		counter++
		item = item.child

		if item == nil {
			break
		}
	}

	return counter
}

func (m *memoryCache) addConfig(config ConfigReaderInterface) {
	m.config = config
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
				fmt.Println("Clean cache running", time.Now().Format(time.Kitchen))
				m.unlinkExpiredCache()
				fmt.Println("Clean cache finished", time.Now().Format(time.Kitchen))
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

	var expirationDuration time.Duration

	for k, item := range m.items {
		path := strings.Split(k, cachePathKeySeparator)[0]
		if m.config != nil {
			expirationDuration = m.config.GetTimeFrameDurationToCheckRequests(path)
		}
		for {
			if expirationDuration == 0 {
				expirationDuration = item.expiryDuration
			}

			if time.Now().After(item.creationTime.Add(expirationDuration)) {
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

			item = item.child
		}
	}
}
