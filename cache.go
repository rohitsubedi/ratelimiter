package ratelimiter

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type cacheType string

var (
	cacheTypeMemory    cacheType = "memory"
	cacheTypeRedis     cacheType = "redis"
	defaultExpiration            = 0 * time.Second // cache will never expire unless killed
	errConnectingRedis           = fmt.Errorf("cache lib: cannot connect to redis server")
)

type cache struct {
	mu          sync.RWMutex
	cacheType   cacheType
	items       map[string]map[string]int64
	redisClient *redis.Client
	cleaner     *cacheCleaner
}

type cacheCleaner struct {
	interval *time.Timer
	stop     chan bool
}

// Creates new memory cache. 0*time.Second indicates the cache will never expire
func newMemoryCache(cleanerTime time.Duration) *cache {
	var cleaner *cacheCleaner
	if cleanerTime > 0 {
		cleaner = &cacheCleaner{
			interval: time.NewTimer(cleanerTime),
			stop:     make(chan bool),
		}
	}

	cache := &cache{
		cacheType: cacheTypeMemory,
		items:     make(map[string]map[string]int64),
		cleaner:   cleaner,
	}

	cache.cleanExpiredMemoryCache(cleanerTime)

	return cache
}

// Creates new redis cache. 0*time.Second indicates the cache will never expire
func newRedisCache(host, password string) (*cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("%v: %w", errConnectingRedis, err)
	}

	return &cache{
		cacheType:   cacheTypeRedis,
		redisClient: client,
	}, nil
}

// This will set the counter to the key depending on the cache type user selects (memory, redis).
// If cache already exists for given key, it will override the counter with the latest expiration. Returns error if there are any
func (c *cache) appendEntry(key string, expirationTime time.Duration) error {
	var expiration int64
	if expirationTime > defaultExpiration {
		expiration = time.Now().Add(expirationTime).UnixNano()
	}

	switch c.cacheType {
	case cacheTypeMemory:
		c.mu.Lock()
		defer c.mu.Unlock()

		if v, ok := c.items[key]; ok {
			v[uuid.NewString()] = expiration
			c.items[key] = v
			return nil
		}

		v := make(map[string]int64)
		v[uuid.NewString()] = expiration
		c.items[key] = v
	case cacheTypeRedis:
		keyWithPrefix := fmt.Sprintf("%s||%s", uuid.NewString(), key)
		if err := c.redisClient.Set(context.Background(), keyWithPrefix, uuid.NewString(), expirationTime).Err(); err != nil {
			return err
		}
	}

	return nil
}

// This returns the counter in the cache for the given key. Return 0 if it doesn't exist. Returns error if exists
func (c *cache) getValidCacheCount(key string) (int64, error) {
	switch c.cacheType {
	case cacheTypeMemory:
		return c.getValidMemoryCacheCount(key)
	case cacheTypeRedis:
		return c.getCountRedisCache(key)
	}

	return 0, nil
}

// Returns counter from redis cache for given key. Returns 0 if cache is deleted due to expiry. Returns error if counter in cache is not a number
func (c *cache) getCountRedisCache(key string) (int64, error) {
	ctx := context.Background()
	iter := c.redisClient.Scan(ctx, 0, fmt.Sprintf("*||%s", key), 0).Iterator()
	count := 0
	for iter.Next(ctx) {
		count++
	}

	if err := iter.Err(); err != nil {
		return 0, err
	}

	return int64(count), nil
}

// This returns the counter in the cache for the given key. Return 0 if it doesn't exist. Returns error if exists
func (c *cache) getValidMemoryCacheCount(key string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items, found := c.items[key]
	if !found {
		return 0, nil
	}

	counter := 0
	for _, expiration := range items {
		if expiration > 0 && time.Now().UnixNano() > expiration {
			continue
		}
		counter++
	}

	return int64(counter), nil
}

// This is a job that will execute each duration of the cache and clears the expired cache
func (c *cache) cleanExpiredMemoryCache(cleanerTime time.Duration) {
	if c.cleaner == nil {
		return
	}

	runtime.SetFinalizer(c.cleaner, stopCleaningRoutine)

	go func() {
		for {
			select {
			case <-c.cleaner.interval.C:
				keysToDelete := c.getKeysToDelete()
				c.batchDeleteFromMap(keysToDelete)

				c.cleaner.interval.Reset(cleanerTime)
			case <-c.cleaner.stop:
				c.cleaner.interval.Stop()
			}
		}
	}()
}

// go routine is stopped when stop is set to true
func stopCleaningRoutine(cleaner *cacheCleaner) {
	cleaner.stop <- true
}

// get keys of all the cache that are expired
func (c *cache) getKeysToDelete() map[string][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keysToDelete := make(map[string][]string)

	for k, values := range c.items {
		keysToDelete[k] = []string{}

		for valKey, expiration := range values {
			if expiration > 0 && time.Now().UnixNano() > expiration {
				keysToDelete[k] = append(keysToDelete[k], valKey)
			}
		}
	}

	return keysToDelete
}

// delete all keys from the memory cache
func (c *cache) batchDeleteFromMap(keyMap map[string][]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, subKeys := range keyMap {
		for _, subKey := range subKeys {
			delete(c.items[k], subKey)
		}

		if len(c.items[k]) == 0 {
			delete(c.items, k)
		}
	}

}
