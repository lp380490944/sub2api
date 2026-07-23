package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestBedrockFailureCounter(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := NewBedrockFailureCounterCache(rdb)
	ctx := context.Background()

	n1, err := cache.IncrementBedrockFailureCount(ctx, 42, 300)
	if err != nil || n1 != 1 {
		t.Fatalf("first incr = %d, %v; want 1", n1, err)
	}
	n2, _ := cache.IncrementBedrockFailureCount(ctx, 42, 300)
	if n2 != 2 {
		t.Fatalf("second incr = %d; want 2", n2)
	}
	if err := cache.ResetBedrockFailureCount(ctx, 42); err != nil {
		t.Fatalf("reset: %v", err)
	}
	n3, _ := cache.IncrementBedrockFailureCount(ctx, 42, 300)
	if n3 != 1 {
		t.Fatalf("after reset incr = %d; want 1", n3)
	}
}
