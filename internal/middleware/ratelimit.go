package middleware

import (
	"cleanmark/pkg/response"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	limiters map[string]*BucketLimiter
	mu       sync.RWMutex
}

type BucketLimiter struct {
	tokens     int
	maxTokens  int
	lastRefill time.Time
	rate       time.Duration
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*BucketLimiter),
	}
}

func (rl *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	
	if limiter, exists := rl.limiters[key]; exists {
		elapsed := now.Sub(limiter.lastRefill)
		tokensToAdd := int(elapsed / limiter.rate)
		
		if tokensToAdd > 0 {
			limiter.tokens = min(limiter.maxTokens, limiter.tokens+tokensToAdd)
			limiter.lastRefill = now
		}
		
		if limiter.tokens > 0 {
			limiter.tokens--
			return true
		}
		return false
	}

	rl.limiters[key] = &BucketLimiter{
		tokens:     limit - 1,
		maxTokens:  limit,
		lastRefill: now,
		rate:       window / time.Duration(limit),
	}
	return true
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, limiter := range rl.limiters {
		if now.Sub(limiter.lastRefill) > time.Hour*1 {
			delete(rl.limiters, key)
		}
	}
}

var globalLimiter = NewRateLimiter()

func RateLimit(requestsPerMinute int) gin.HandlerFunc {
	window := time.Minute
	go func() {
		ticker := time.NewTicker(time.Minute * 5)
		for range ticker.C {
			globalLimiter.Cleanup()
		}
	}()

	return func(c *gin.Context) {
		key := c.ClientIP()
		
		if !globalLimiter.Allow(key, requestsPerMinute, window) {
			response.Error(c, 429, 429, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		
		c.Next()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
