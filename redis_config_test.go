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
