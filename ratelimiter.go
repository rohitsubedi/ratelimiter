package ratelimiter

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	ErrMsgGettingValueFromCache    = "Error getting value from cache"
	ErrMsgPossibleBruteForceAttack = "Possible Brute Force Attack"

	errMsgWritingValueFromCache     = "Error writing value to cache"
	msgRateLimitValueEmpty          = "Rate Limit value is empty"
	msgRateLimitValueFuncIsNil      = "Rate Limit value function is nil"
	msgRateLimitValueSkipped        = "Rate Limit check skipped"
	defaultTimeToCheckForRateLimit  = 10 * time.Minute
	maxNumberOfRequestAllowedInTime = int64(100)
)

type HandlerWrapperFunc func(res http.ResponseWriter, req *http.Request)
type HttpResponseFunc func(w http.ResponseWriter, message string)
type RateLimitValueFunc func(req *http.Request) string

type ConfigReaderInterface interface {
	GetTimeFrameDurationToCheckRequests(path string) time.Duration
	GetMaxRequestAllowedPerTimeFrame(path string) int64
	ShouldSkipRateLimitCheck(rateLimitValue string) bool
}

type LeveledLogger interface {
	Error(args ...interface{})
	Info(args ...interface{})
}

type Limiter interface {
	RateLimit(
		fn HandlerWrapperFunc,
		urlPath string,
		config ConfigReaderInterface,
		errorResponse HttpResponseFunc,
		rateLimitValueFunc RateLimitValueFunc,
		logger interface{},
	) HandlerWrapperFunc
}

type limiter struct {
	cache *cache
}

func NewRateLimiterUsingMemory(cacheCleaningInterval time.Duration) Limiter {
	if cacheCleaningInterval < 0 {
		cacheCleaningInterval = 0
	}

	return &limiter{
		cache: newMemoryCache(cacheCleaningInterval),
	}
}

func NewRateLimiterUsingRedis(redisConfig *RedisConfig) (Limiter, error) {
	redisCache, err := newRedisCache(redisConfig.GetHost(), redisConfig.GetPassword())
	if err != nil {
		return nil, err
	}

	rateLimiter := &limiter{
		cache: redisCache,
	}

	return rateLimiter, nil
}

func (l *limiter) RateLimit(
	fn HandlerWrapperFunc,
	urlPath string,
	config ConfigReaderInterface,
	errorResponse HttpResponseFunc,
	rateLimitValueFunc RateLimitValueFunc,
	logger interface{},
) HandlerWrapperFunc {
	if fn == nil {
		log.Fatal("Empty handler wrapper function")
	}

	return func(writer http.ResponseWriter, req *http.Request) {
		if rateLimitValueFunc == nil {
			logInfo(logger, msgRateLimitValueFuncIsNil)
			fn(writer, req)

			return
		}

		rateLimitValue := rateLimitValueFunc(req)
		if rateLimitValue == "" {
			logInfo(logger, msgRateLimitValueEmpty)
			fn(writer, req)

			return
		}
		// Check if config says that the rateLimit value should skip rateLimiter check
		if config != nil && config.ShouldSkipRateLimitCheck(rateLimitValue) {
			logInfo(logger, msgRateLimitValueSkipped)
			fn(writer, req)

			return
		}

		cacheKey := fmt.Sprintf("%s:%s", urlPath, rateLimitValue)

		val, err := l.cache.getValidCacheCount(cacheKey)
		if err != nil {
			logError(logger, ErrMsgGettingValueFromCache)
			predefinedResponse(errorResponse, writer, ErrMsgGettingValueFromCache)

			return
		}

		maxRequestAllowedInTimeFrame := maxNumberOfRequestAllowedInTime
		if config != nil {
			maxRequestAllowedInTimeFrame = config.GetMaxRequestAllowedPerTimeFrame(urlPath)
		}

		if val >= maxRequestAllowedInTimeFrame {
			logError(logger, ErrMsgPossibleBruteForceAttack)
			predefinedResponse(errorResponse, writer, ErrMsgPossibleBruteForceAttack)

			return
		}

		defaultTimeFrameDurationToCheck := defaultTimeToCheckForRateLimit
		if config != nil {
			defaultTimeFrameDurationToCheck = config.GetTimeFrameDurationToCheckRequests(urlPath)
		}

		if err := l.cache.appendEntry(cacheKey, defaultTimeFrameDurationToCheck); err != nil {
			logError(logger, errMsgWritingValueFromCache)
		}

		fn(writer, req)
	}
}

func predefinedResponse(response HttpResponseFunc, writer http.ResponseWriter, msg string) {
	if response != nil {
		response(writer, msg)
		return
	}

	writer.WriteHeader(http.StatusBadRequest)
	writer.Write([]byte(msg))
}

func logInfo(logger interface{}, msg string) {
	switch l := logger.(type) {
	case LeveledLogger:
		l.Info(msg)
	default:
		log.Println(msg)
	}
}

func logError(logger interface{}, msg string) {
	switch l := logger.(type) {
	case LeveledLogger:
		l.Error(msg)
	default:
		log.Println(msg)
	}
}
