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

	msgRateLimitValueEmpty          = "Rate Limit value is empty"
	msgRateLimitValueFuncIsNil      = "Rate Limit value function is nil"
	msgRateLimitValueSkipped        = "Rate Limit check skipped"
	defaultTimeToCheckForRateLimit  = 10 * time.Minute
	maxNumberOfRequestAllowedInTime = 100
)

type HandlerWrapperFunc func(res http.ResponseWriter, req *http.Request)
type HttpResponseFunc func(w http.ResponseWriter, message string)
type RateLimitKeyFunc func(req *http.Request) string

type ConfigReaderInterface interface {
	GetTimeFrameDurationToCheckRequests(path string) time.Duration
	GetMaxRequestAllowedPerTimeFrame(path string) int
	ShouldSkipRateLimitCheck(rateLimitKey string) bool
}

type cacheInterface interface {
	AppendEntry(key string, expirationTime time.Duration) error
	GetCount(key string) (int, error)
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
		rateLimitValueFunc RateLimitKeyFunc,
	) HandlerWrapperFunc
	SetLogger(logger LeveledLogger)
}

type limiter struct {
	cache  cacheInterface
	logger interface{}
}

func NewRateLimiterUsingMemory(cacheCleaningInterval time.Duration) Limiter {
	if cacheCleaningInterval < 0 {
		cacheCleaningInterval = 0
	}

	return &limiter{
		cache:  newMemoryCache(cacheCleaningInterval),
		logger: log.Default(),
	}
}

func NewRateLimiterUsingRedis(redisConfig *RedisConfig) (Limiter, error) {
	redisCache, err := newRedisCache(redisConfig.getHost(), redisConfig.getPassword())
	if err != nil {
		return nil, err
	}

	rateLimiter := &limiter{
		cache:  redisCache,
		logger: log.Default(),
	}

	return rateLimiter, nil
}

// SetLogger sets your own logger. Pass nil if you don't want to log anything
func (l *limiter) SetLogger(logger LeveledLogger) {
	l.logger = logger
}

func (l *limiter) RateLimit(
	fn HandlerWrapperFunc,
	path string,
	config ConfigReaderInterface,
	errorResponse HttpResponseFunc,
	rateLimitKeyFunc RateLimitKeyFunc,
) HandlerWrapperFunc {
	if fn == nil {
		log.Fatal("Empty handler wrapper function")
	}

	return func(writer http.ResponseWriter, req *http.Request) {
		maxRequestAllowedInTimeFrame := maxNumberOfRequestAllowedInTime
		if config != nil {
			maxRequestAllowedInTimeFrame = config.GetMaxRequestAllowedPerTimeFrame(path)
		}

		defaultTimeFrameDurationToCheck := defaultTimeToCheckForRateLimit
		if config != nil {
			defaultTimeFrameDurationToCheck = config.GetTimeFrameDurationToCheckRequests(path)
		}

		if rateLimitKeyFunc == nil {
			logInfo(l.logger, msgRateLimitValueFuncIsNil)
			fn(writer, req)

			return
		}

		rateLimitKey := rateLimitKeyFunc(req)
		if rateLimitKey == "" {
			logInfo(l.logger, msgRateLimitValueEmpty)
			fn(writer, req)

			return
		}
		// Check if config says that the rateLimit value should skip rateLimiter check
		if config != nil && config.ShouldSkipRateLimitCheck(rateLimitKey) {
			logInfo(l.logger, msgRateLimitValueSkipped)
			fn(writer, req)

			return
		}

		cacheKey := fmt.Sprintf("%s:%s", path, rateLimitKey)

		val, err := l.cache.GetCount(cacheKey)
		if err != nil {
			logError(l.logger, ErrMsgGettingValueFromCache)
			predefinedResponse(errorResponse, writer, ErrMsgGettingValueFromCache)

			return
		}

		_ = l.cache.AppendEntry(cacheKey, defaultTimeFrameDurationToCheck)

		if val >= maxRequestAllowedInTimeFrame {
			logError(l.logger, ErrMsgPossibleBruteForceAttack)
			predefinedResponse(errorResponse, writer, ErrMsgPossibleBruteForceAttack)

			return
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
	if logger == nil {
		return
	}

	switch l := logger.(type) {
	case LeveledLogger:
		l.Info(msg)
	default:
		log.Println(msg)
	}
}

func logError(logger interface{}, msg string) {
	if logger == nil {
		return
	}

	switch l := logger.(type) {
	case LeveledLogger:
		l.Error(msg)
	default:
		log.Println(msg)
	}
}
