package ratelimiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	errConnectingRedis = fmt.Errorf("cache lib: cannot connect to redis server")
)

type RedisConfig struct {
	Host     string
	Password string
}

func (r *RedisConfig) getHost() string {
	if r == nil {
		return ""
	}

	return r.Host
}

func (r *RedisConfig) getPassword() string {
	if r == nil {
		return ""
	}

	return r.Password
}

type redisCache struct {
	redisClient *redis.Client
}

func newRedisCache(host, password string) (cacheInterface, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("%v: %w", errConnectingRedis, err)
	}

	return &redisCache{
		redisClient: client,
	}, nil
}

func (r *redisCache) AppendEntry(key string, expirationTime time.Duration) error {
	prevValue, err := r.getByKey(key)
	if err != nil {
		return err
	}

	if err := r.redisClient.Set(context.Background(), key, prevValue+1, expirationTime).Err(); err != nil {
		return err
	}

	return nil
}

func (r *redisCache) GetCount(key string) (int, error) {
	value, err := r.getByKey(key)
	if err != nil {
		return 0, err
	}

	return value, nil
}

func (r *redisCache) getByKey(key string) (int, error) {
	value := r.redisClient.Get(context.Background(), key).Val()
	if value == "" {
		return 0, nil
	}

	return strconv.Atoi(value)
}
