package client

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRateLimiter_NoOp(t *testing.T) {
	// Test that zero requests per second creates a noop limiter
	limiter := NewRateLimiter(0, 0)
	if limiter == nil {
		t.Fatal("expected limiter to be created")
	}

	// Noop limiter should return immediately
	start := time.Now()
	if err := limiter.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 10*time.Millisecond {
		t.Errorf("noop limiter took too long: %v", elapsed)
	}
}

func TestNewRateLimiter_NegativeRequests(t *testing.T) {
	// Test that negative requests per second creates a noop limiter
	limiter := NewRateLimiter(-1, 0)
	if limiter == nil {
		t.Fatal("expected limiter to be created")
	}

	// Should be a noop limiter
	if err := limiter.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRateLimiter_TokenBucket(t *testing.T) {
	// Test token bucket limiter with 10 RPS
	limiter := NewRateLimiter(10, 5)
	if limiter == nil {
		t.Fatal("expected limiter to be created")
	}

	// First request should be fast (burst capacity)
	start := time.Now()
	if err := limiter.Wait(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("first request took too long: %v", elapsed)
	}
}

func TestNewRateLimiter_BurstDefault(t *testing.T) {
	// Test that burst defaults to requests per second
	limiter := NewRateLimiter(5, 0) // burst = 0, should default to 5
	if limiter == nil {
		t.Fatal("expected limiter to be created")
	}

	// Should allow burst requests
	for i := 0; i < 5; i++ {
		if err := limiter.Wait(); err != nil {
			t.Fatalf("unexpected error on request %d: %v", i, err)
		}
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	// Test concurrent access to the rate limiter
	limiter := NewRateLimiter(100, 10)

	var wg sync.WaitGroup
	var errors int64

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := limiter.Wait(); err != nil {
				atomic.AddInt64(&errors, 1)
			}
		}()
	}

	wg.Wait()

	if errors > 0 {
		t.Errorf("unexpected errors: %d", errors)
	}
}

func TestRateLimiter_RateLimiting(t *testing.T) {
	// Test that rate limiting actually works
	// 5 requests per second with burst of 1
	limiter := NewRateLimiter(5, 1)

	start := time.Now()

	// Make 3 requests
	for i := 0; i < 3; i++ {
		if err := limiter.Wait(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	elapsed := time.Since(start)

	// With 5 RPS and burst of 1, 3 requests should take at least 400ms
	// (first request is immediate, then 2 requests at 200ms intervals)
	if elapsed < 300*time.Millisecond {
		t.Errorf("rate limiting not working properly, elapsed: %v", elapsed)
	}
}