# ratelimiter
RateLimiter helps ratelimit the http request based on the config and value to check defined by the user.
For eg. Possible brute force can be detected and request will not be passed through the service



## Installation
    go get github.com/rohitsubedi/ratelimiter@v1.3.0

## Testing
    make test

## Avalilable methods
### RateLimit using system memory
```go
limiter := ratelimiter.NewRateLimiterUsingMemory(cacheCleaningInterval)
limiter.RateLimit(handlerFunc, path, config, errorResponseFunc, rateLimitValueFunc)

handlerFunc = func(res http.ResponseWriter, req *http.Request) // Your http handler
path = string (Name of your handler/ identifier)
config = This must follow the ConfigReaderInterface. ConfigReaderInterface has 3 methods
    GetTimeFrameDurationToCheckRequests(path string) time.Duration // This returns the time frame in which the total request is counted
    GetMaxRequestAllowedPerTimeFrame(path string) int64 // Total request allwed in the time frame. If request exceed this value, it will execute errorResponseFunc
    ShouldSkipRateLimitCheck(path, rateLimitValue string) bool // Skip rate limit if return true
	*If passed nil, default values will be used (100 request allowed per 10 minutes)
errorResponseFunc = func(w http.ResponseWriter, message string) // If the request is rate limited, this method will be called
rateLimitValueFunc = func(req *http.Request) string // This should return the value that needs to be ratelimited. For eg ip address from the request
```
### RateLimit using redis
```go
limiter, err := ratelimiter.NewRateLimiterUsingRedis(redisHost, redisPassword)
limiter.RateLimit(handlerFunc, path, config, errorResponseFunc, rateLimitValueFunc)
//Every parameter is same as above

```
```go
//In both of the above cases, the error and infos are logged with default go logger. You can either disable the logs
//or initialize your own log the follows the following interface
type LeveledLogger interface {
    Error(args ...interface{})
    Info(args ...interface{})
}
```

## Example
```golang
package main

import (
    "log"
    "net/http"
    "time"

    "github.com/rohitsubedi/ratelimiter"
)

// This is the configuration that you will define
const skipRateLimitValue = "1234"
type config struct{}
// This returns the timeframe to check. The path parameter helps to define different value if needed
func (c *config) GetTimeFrameDurationToCheckRequests(path string) time.Duration {
    return 10 * time.Second // Pass 0 for infinity
}
// This returns the max requst allwed in the time frame. The path parameter helps to define different value if needed
func (c *config) GetMaxRequestAllowedPerTimeFrame(path string) int64 {
    return 10
}
// This will help skip the rate limit check. For eg: The stage server ip can be added in the ignore list for rate limit check
func (c *config) ShouldSkipRateLimitCheck(rateLimitValue string) bool {
    return rateLimitValue == skipRateLimitValue
}

// This method will be executed if rate limit is exceeded. User can define what to return to user in such scenario
var errorResp = func(w http.ResponseWriter, message string) {
    w.WriteHeader(http.StatusBadRequest)
    w.Write([]byte(message))
}

// This method is defined by the user. This value will be rate limited by ratelimiter package.
// In this case the ipAddress will be rate limited meaning, from the same ip only X number of request is allowed in Y time frame
var rateLimitValueFunc = func(req *http.Request) string {
    return req.Header.Get("ip-address")
}

func main() { 
// The handler define by the user
    handlerFunc := func(writer http.ResponseWriter, request *http.Request) {
        writer.WriteHeader(http.StatusNoContent)
    }

    // Cache cleaning will run every 15 minute to stop memory leak
    // Pass 0 if you don't want to clean the cache
    memoryLimiter := ratelimiter.NewRateLimiterUsingMemory(15 * time.Minute) 
    // You can set logger to nil if you dont want to ratelimiter to log anything
    // You can also set looger to your own logger that follows LeveledLogger interface
    memoryLimiter.SetLogger(nil)
    redisLimiter, err := ratelimiter.NewRateLimiterUsingRedis("0.0.0.0:6379", "redis_password")
    if err != nil {
        log.Fatalf("Error creating limiter: %v", err)
    }
    // Rate limit the handlerFunc by ratelimiter
    http.HandleFunc("/", memoryLimiter.RateLimit(handlerFunc, "HomePage", new(config), errorResp, rateLimitValueFunc))
    http.HandleFunc("/about-us", redisLimiter.RateLimit(handlerFunc, "AboutPage", new(config), errorResp, rateLimitValueFunc))

    log.Fatalln(http.ListenAndServe(":8080", nil))
}
```

