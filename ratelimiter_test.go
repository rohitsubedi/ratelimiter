package ratelimiter

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	statusCodeSuccess    = http.StatusOK
	statusCodeBruteForce = http.StatusBadRequest
	skipRateLimitValue   = "123"
)

type config struct{}

func (c *config) GetTimeFrameDurationToCheckRequests(path string) time.Duration {
	return 10 * time.Second
}

func (c *config) GetMaxRequestAllowedPerTimeFrame(path string) int64 {
	return 10
}

func (c *config) ShouldSkipRateLimitCheck(rateLimitValue string) bool {
	return rateLimitValue == skipRateLimitValue
}

func TestRateLimitInMemory(t *testing.T) {
	rateLimitValue := uuid.New().String()
	handlerFunc := func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(statusCodeSuccess)
	}
	errorResp := func(w http.ResponseWriter, message string) {
		w.WriteHeader(statusCodeBruteForce)
		w.Write([]byte(message))
	}
	limitValFunc := func(req *http.Request) string {
		return rateLimitValue
	}

	conf := new(config)
	rateLimiter := NewRateLimiterUsingMemory(conf.GetTimeFrameDurationToCheckRequests(""))
	wrapper := rateLimiter.RateLimit(handlerFunc, "Login", conf, errorResp, limitValFunc, nil)

	for i := 0; i < int(2*conf.GetMaxRequestAllowedPerTimeFrame("")); i++ {
		resp := &httptest.ResponseRecorder{Body: new(bytes.Buffer)}
		wrapper(resp, new(http.Request))

		if int64(i) >= conf.GetMaxRequestAllowedPerTimeFrame("") {
			assert.Equal(t, resp.Code, statusCodeBruteForce)
			assert.Equal(t, resp.Body.Bytes(), []byte(ErrMsgPossibleBruteForceAttack))
			continue
		}

		assert.Equal(t, resp.Code, statusCodeSuccess)
	}

	time.Sleep(conf.GetTimeFrameDurationToCheckRequests("")) // cache is deleted so the request can be made again
	resp := new(httptest.ResponseRecorder)
	wrapper(resp, new(http.Request))
	assert.Equal(t, resp.Code, statusCodeSuccess)
}

func TestRateLimitInMemoryEmptyConfig(t *testing.T) {
	rateLimitValue := uuid.New().String()
	handlerFunc := func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(statusCodeSuccess)
	}
	errorResp := func(w http.ResponseWriter, message string) {
		w.WriteHeader(statusCodeBruteForce)
		w.Write([]byte(message))
	}
	limitValFunc := func(req *http.Request) string {
		return rateLimitValue
	}

	rateLimiter := NewRateLimiterUsingMemory(10 * time.Minute)
	wrapper := rateLimiter.RateLimit(handlerFunc, "Login", nil, errorResp, limitValFunc, nil)

	// All request should pass as config is nil
	for i := 0; i < 50; i++ {
		resp := &httptest.ResponseRecorder{Body: new(bytes.Buffer)}
		wrapper(resp, new(http.Request))
		assert.Equal(t, resp.Code, statusCodeSuccess)
	}
}

func TestRateLimitInMemoryRateLimiterValFuncNil(t *testing.T) {
	handlerFunc := func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(statusCodeSuccess)
	}
	errorResp := func(w http.ResponseWriter, message string) {
		w.WriteHeader(statusCodeBruteForce)
		w.Write([]byte(message))
	}
	limitValFunc := func(req *http.Request) string {
		return ""
	}

	rateLimiter := NewRateLimiterUsingMemory(10 * time.Minute)
	wrapper := rateLimiter.RateLimit(handlerFunc, "Login", nil, errorResp, limitValFunc, nil)

	// All request should pass as rate limiter value func is nil
	for i := 0; i < 50; i++ {
		resp := &httptest.ResponseRecorder{Body: new(bytes.Buffer)}
		wrapper(resp, new(http.Request))
		assert.Equal(t, resp.Code, statusCodeSuccess)
	}
}

func TestRateLimitInMemoryRateLimiterValFuncReturnsEmpty(t *testing.T) {
	handlerFunc := func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(statusCodeSuccess)
	}
	errorResp := func(w http.ResponseWriter, message string) {
		w.WriteHeader(statusCodeBruteForce)
		w.Write([]byte(message))
	}

	rateLimiter := NewRateLimiterUsingMemory(10 * time.Minute)
	wrapper := rateLimiter.RateLimit(handlerFunc, "Login", nil, errorResp, nil, nil)

	// All request should pass as rate limiter value func is nil
	for i := 0; i < 50; i++ {
		resp := &httptest.ResponseRecorder{Body: new(bytes.Buffer)}
		wrapper(resp, new(http.Request))
		assert.Equal(t, resp.Code, statusCodeSuccess)
	}
}

func TestRateLimitInMemory_SkipRateLimit(t *testing.T) {
	handlerFunc := func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(statusCodeSuccess)
	}
	errorResp := func(w http.ResponseWriter, msg string) {
		w.WriteHeader(statusCodeBruteForce)
		w.Write([]byte(msg))
	}
	limitValFunc := func(req *http.Request) string {
		return skipRateLimitValue
	}

	conf := new(config)
	rateLimiter := NewRateLimiterUsingMemory(conf.GetTimeFrameDurationToCheckRequests(""))
	wrapper := rateLimiter.RateLimit(handlerFunc, "Login", conf, errorResp, limitValFunc, nil)

	// All request should pass even the request is double the threshold
	for i := 0; i < int(2*conf.GetMaxRequestAllowedPerTimeFrame("")); i++ {
		resp := &httptest.ResponseRecorder{Body: new(bytes.Buffer)}
		wrapper(resp, new(http.Request))
		assert.Equal(t, resp.Code, statusCodeSuccess)
	}
}

// To run this test run this command first: "docker-compose up -d"
func TestRateLimitInRedis(t *testing.T) {
	rateLimitValue := uuid.New().String()
	handlerFunc := func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(statusCodeSuccess)
	}
	limitValFunc := func(req *http.Request) string {
		return rateLimitValue
	}

	rateLimiter, err := NewRateLimiterUsingRedis(&RedisConfig{
		Host:     "0.0.0.0:6379",
		Password: "redis_password",
	})
	assert.NoError(t, err)

	conf := new(config)
	wrapper := rateLimiter.RateLimit(handlerFunc, "Login", conf, nil, limitValFunc, nil)

	for i := 0; i < int(2*conf.GetMaxRequestAllowedPerTimeFrame("")); i++ {
		resp := &httptest.ResponseRecorder{Body: new(bytes.Buffer)}
		wrapper(resp, new(http.Request))

		if int64(i) >= conf.GetMaxRequestAllowedPerTimeFrame("") {
			assert.Equal(t, resp.Code, statusCodeBruteForce)
			assert.Equal(t, resp.Body.Bytes(), []byte(ErrMsgPossibleBruteForceAttack))
			continue
		}

		assert.Equal(t, resp.Code, statusCodeSuccess)
	}

	time.Sleep(conf.GetTimeFrameDurationToCheckRequests("")) // cache is deleted so the request can be made again
	resp := new(httptest.ResponseRecorder)
	wrapper(resp, new(http.Request))
	assert.Equal(t, resp.Code, statusCodeSuccess)
}

// To run this test run this command first: "docker-compose up -d"
func TestRateLimitInRedis_SkipRateLimit(t *testing.T) {
	handlerFunc := func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(statusCodeSuccess)
	}
	errorResp := func(w http.ResponseWriter, msg string) {
		w.WriteHeader(statusCodeBruteForce)
		w.Write([]byte(msg))
	}
	limitValFunc := func(req *http.Request) string {
		return skipRateLimitValue
	}

	rateLimiter, err := NewRateLimiterUsingRedis(&RedisConfig{
		Host:     "0.0.0.0:6379",
		Password: "redis_password",
	})
	assert.NoError(t, err)

	conf := new(config)
	wrapper := rateLimiter.RateLimit(handlerFunc, "Login", conf, errorResp, limitValFunc, nil)

	// All request should pass even the request is double the threshold
	for i := 0; i < int(2*conf.GetMaxRequestAllowedPerTimeFrame("")); i++ {
		resp := &httptest.ResponseRecorder{Body: new(bytes.Buffer)}
		wrapper(resp, new(http.Request))
		assert.Equal(t, resp.Code, statusCodeSuccess)
	}
}
