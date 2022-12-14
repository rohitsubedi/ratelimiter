package ratelimiter

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	ErrMsgPossibleBruteForceAttack = "Possible Brute Force Attack"

	msgRateLimitValueEmpty          = "Rate Limit value is empty"
	msgRateLimitValueFuncIsNil      = "Rate Limit value function is nil"
	msgRateLimitValueSkipped        = "Rate Limit check skipped"
	defaultTimeToCheckForRateLimit  = 10 * time.Minute
	maxNumberOfRequestAllowedInTime = 100

	cachePathKeySeparator = "||||"
)

type HttpResponseFunc func(w http.ResponseWriter, message string)
type RateLimitKeyFunc func(req *http.Request) string

type ConfigReaderInterface interface {
	GetTimeFrameDurationToCheckRequests(path string) time.Duration
	GetMaxRequestAllowedPerTimeFrame(path string) int
	ShouldSkipRateLimitCheck(path, rateLimitKey string) bool
}

type cacheInterface interface {
	appendEntry(key string, expirationTime time.Duration) error
	getCount(key string, expirationTime time.Duration) int
}

type LeveledLogger interface {
	Error(args ...interface{})
	Info(args ...interface{})
}

type Limiter interface {
	RateLimit(
		fn http.HandlerFunc,
		path string,
		config ConfigReaderInterface,
		errorResponse HttpResponseFunc,
		rateLimitValueFunc RateLimitKeyFunc,
	) http.HandlerFunc
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

func NewRateLimiterUsingRedis(host, password string) (Limiter, error) {
	redisCache, err := newRedisCache(host, password)
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
	fn http.HandlerFunc,
	path string,
	config ConfigReaderInterface,
	errorResponse HttpResponseFunc,
	rateLimitKeyFunc RateLimitKeyFunc,
) http.HandlerFunc {
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
		if config != nil && config.ShouldSkipRateLimitCheck(path, rateLimitKey) {
			logInfo(l.logger, msgRateLimitValueSkipped)
			fn(writer, req)

			return
		}

		cacheKey := fmt.Sprintf("%s%s%s", path, cachePathKeySeparator, rateLimitKey)
		limitCount := l.cache.getCount(cacheKey, defaultTimeFrameDurationToCheck)

		if limitCount >= maxRequestAllowedInTimeFrame {
			logError(l.logger, ErrMsgPossibleBruteForceAttack)
			predefinedResponse(errorResponse, writer, ErrMsgPossibleBruteForceAttack)

			return
		}

		_ = l.cache.appendEntry(cacheKey, defaultTimeFrameDurationToCheck)

		fn(writer, req)
	}
}

func predefinedResponse(response HttpResponseFunc, writer http.ResponseWriter, msg string) {
	if response != nil {
		response(writer, msg)
		return
	}

	writer.WriteHeader(http.StatusBadRequest)
	_, _ = writer.Write([]byte(msg))
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
