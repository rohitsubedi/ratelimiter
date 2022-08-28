# ratelimiter

Very esay golang package for ratelimiting the http endpoints based on the value in the request

## Installation
    go get github.com/rohitsubedi/ratelimiter@v1.0.0

## Description
RateLimiter helps ratelimit the http request based on the config and value to check defined by the user.
For eg. Possible brute force can be detected and request will not be passed through the service

## Avalilable methods
### RateLimit using system memory
    limiter := ratelimiter.NewRateLimiterUsingMemory(cacheCleaningInterval time.Duration)
    limiter.RateLimit(handlerFunc, urlPath, config, errorResponseFunc, rateLimitValueFunc, logger)
    
    handlerFunc = func(res http.ResponseWriter, req *http.Request) // Your http handler
    urlPath = string (Name of your handler)
    config = This must follow the ConfigReaderInterface
        ConfigReaderInterface has 3 methods
        GetTimeFrameDurationToCheckRequests(path string) time.Duration // This returns the time frame in which the total request is counted
        GetMaxRequestAllowedPerTimeFrame(path string) int64 // Total request allwed in the time frame. If request exceed this value, it will execute errorResponseFunc
    errorResponseFunc = func(w http.ResponseWriter, message string) // If the request is rate limited, this method will be called
    rateLimitValueFunc = func(req *http.Request) string // This should return the value that needs to be ratelimited. For eg ip address from the request
    logger = This can be nil or follow the LeveledLogger interface. If nil, then default logger will be used to log info/error
        LeveledLogger has 2 methods
        Error(args ...interface{}) // To log error
        Info(args ...interface{}) // To log info
### RateLimit using redis
    limiter, err := ratelimiter.NewRateLimiterUsingRedis(redisConfig *ratelimiter.RedisConfig)
    limiter.RateLimit(handlerFunc, urlPath, config, errorResponseFunc, rateLimitValueFunc, logger)
    
    Everything is same except for redisConfig
    redisConfig = &ratelimiter.RedisConfig{
        Host     string
	    Password string
    }

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
        redisLimiter, err := ratelimiter.NewRateLimiterUsingRedis(&ratelimiter.RedisConfig{
            Host:     "0.0.0.0:6379",
            Password: "redis_password",
        })
        if err != nil {
            log.Fatalf("Error creating limiter: %v", err)
        }
        // Rate limit the handlerFunc by ratelimiter
        http.HandleFunc("/", memoryLimiter.RateLimit(handlerFunc, "HomePage", new(config), errorResp, rateLimitValueFunc, nil))
        http.HandleFunc("/about-us", redisLimiter.RateLimit(handlerFunc, "AboutPage", new(config), errorResp, rateLimitValueFunc, nil))
    
        log.Fatalln(http.ListenAndServe(":8080", nil))
    }
```

