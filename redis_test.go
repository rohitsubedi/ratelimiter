package ratelimiter

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDefaultRedisCache(t *testing.T) {
	cacheTime := 1 * time.Second
	cacheKey := uuid.New().String()

	c, err := newRedisCache("0.0.0.0:6379", "redis_password")
	assert.NoError(t, err)

	err = c.AppendEntry(cacheKey, cacheTime)
	assert.NoError(t, err)

	val, err := c.GetCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, 1)

	time.Sleep(cacheTime) // cache counter should be deleted
	val, err = c.GetCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, 0)
}

func TestValidConfig(t *testing.T) {
	conf := &RedisConfig{
		Host:     "abc",
		Password: "def",
	}

	assert.Equal(t, "abc", conf.getHost())
	assert.Equal(t, "def", conf.getPassword())
}

func TestNilConfig(t *testing.T) {
	var getConfig = func() *RedisConfig {
		return nil
	}

	conf := getConfig()
	assert.Equal(t, "", conf.getHost())
	assert.Equal(t, "", conf.getPassword())
}
