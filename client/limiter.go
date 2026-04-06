package client

import (
	"sync"
	"time"
)

// RateLimiter 限速器接口
type RateLimiter interface {
	Wait() error
}

// tokenBucketLimiter 令牌桶限速器实现
type tokenBucketLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate int
	lastRefill time.Time
}

// NewRateLimiter 创建限速器
// requestsPerSecond: 每秒允许请求数 (RPS), 0 表示不限速
// burst: 突发请求数 (桶容量), 如果 <=0 则使用 requestsPerSecond
func NewRateLimiter(requestsPerSecond, burst int) RateLimiter {
	if requestsPerSecond <= 0 {
		return &noopLimiter{} // 不限速
	}
	if burst <= 0 {
		burst = requestsPerSecond
	}
	return &tokenBucketLimiter{
		tokens:     burst,
		maxTokens:  burst,
		refillRate: requestsPerSecond,
		lastRefill: time.Now(),
	}
}

// Wait 等待直到可以发送请求
func (l *tokenBucketLimiter) Wait() error {
	for {
		if l.tryConsume() {
			return nil
		}
		time.Sleep(time.Millisecond * 10) // 短暂等待后重试
	}
}

// tryConsume 尝试消费一个令牌
func (l *tokenBucketLimiter) tryConsume() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens > 0 {
		l.tokens--
		return true
	}
	return false
}

// refill 补充令牌
func (l *tokenBucketLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(l.refillRate))

	if tokensToAdd > 0 {
		l.tokens = min(l.maxTokens, l.tokens+tokensToAdd)
		l.lastRefill = now
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// noopLimiter 不限速实现
type noopLimiter struct{}

// Wait 立即返回，不限速
func (l *noopLimiter) Wait() error {
	return nil
}
