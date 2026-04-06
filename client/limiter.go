package client

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimiter 限速器接口
type RateLimiter interface {
	Wait() error
}

// tokenBucketLimiter 令牌桶限速器实现
type tokenBucketLimiter struct {
	limiter *rate.Limiter
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
		limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
	}
}

// Wait 等待直到可以发送请求
func (l *tokenBucketLimiter) Wait() error {
	return l.limiter.Wait(context.Background())
}

// noopLimiter 不限速实现
type noopLimiter struct{}

// Wait 立即返回，不限速
func (l *noopLimiter) Wait() error { return nil }