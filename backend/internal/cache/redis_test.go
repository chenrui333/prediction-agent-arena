package cache

import (
	"context"
	"testing"
	"time"
)

func TestDisabledCacheAllowsRateLimit(t *testing.T) {
	client := New("", "", nil)
	allowed, err := client.Allow(context.Background(), "rate:test", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("disabled cache should allow request")
	}
}

func TestUnavailableRedisAllowsRateLimitWithError(t *testing.T) {
	client := New("127.0.0.1:1", "", nil)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	allowed, err := client.Allow(ctx, "rate:test", 1, time.Minute)
	if err == nil {
		t.Fatal("expected redis error")
	}
	if !allowed {
		t.Fatal("redis outage should not become authoritative for rate limiting")
	}
}
