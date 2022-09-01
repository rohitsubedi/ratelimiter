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
	err := c.AppendEntry(cacheKey, cacheTime)
	assert.NoError(t, err)
	val, err := c.GetCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, 1)

	time.Sleep(cacheTime) // cache counter should be deleted
	val, err = c.GetCount(cacheKey)
	assert.NoError(t, err)
	assert.Equal(t, val, 0)
}
