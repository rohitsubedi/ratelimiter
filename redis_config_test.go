package ratelimiter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidConfig(t *testing.T) {
	conf := &RedisConfig{
		Host:     "abc",
		Password: "def",
	}

	assert.Equal(t, "abc", conf.GetHost())
	assert.Equal(t, "def", conf.GetPassword())
}

func TestNilConfig(t *testing.T) {
	var getConfig = func() *RedisConfig {
		return nil
	}

	conf := getConfig()
	assert.Equal(t, "", conf.GetHost())
	assert.Equal(t, "", conf.GetPassword())
}
