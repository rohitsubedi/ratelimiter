package ratelimiter

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDefaultCache(t *testing.T) {
	cacheTime := 1 * time.Second
	cacheKey := uuid.New().String()

	c := newMemoryCache(cacheTime)
	err := c.appendEntry(cacheKey, cacheTime)
	assert.NoError(t, err)
	val, err := c.getValidCacheCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, int64(1))

	time.Sleep(cacheTime) // cache counter should be deleted
	val, err = c.getValidCacheCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, int64(0))

	time.Sleep(cacheTime) // wait till the map is cleared
	c.mu.RLock()
	defer c.mu.RUnlock()
	assert.Empty(t, c.items[cacheKey])
	assert.Empty(t, c.items)
}

// To run this test run this command first: "docker-compose up -d"
func TestDefaultRedisCache(t *testing.T) {
	cacheTime := 1 * time.Second
	cacheKey := uuid.New().String()

	c, err := newRedisCache("0.0.0.0:6379", "redis_password")
	assert.NoError(t, err)

	err = c.appendEntry(cacheKey, cacheTime)
	assert.NoError(t, err)

	val, err := c.getValidCacheCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, int64(1))

	time.Sleep(cacheTime) // cache counter should be deleted
	val, err = c.getValidCacheCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, int64(0))
}
