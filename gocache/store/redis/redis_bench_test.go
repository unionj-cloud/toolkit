package redis

import (
	"context"
	"fmt"
	"math"
	"testing"

	redis "github.com/redis/go-redis/v9"
	lib_store "github.com/unionj-cloud/toolkit/gocache/lib/store"
)

func BenchmarkRedisSet(b *testing.B) {
	ctx := context.Background()

	store := NewRedis(redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	}))

	for k := 0.; k <= 10; k++ {
		n := int(math.Pow(2, k))
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			for i := 0; i < b.N*n; i++ {
				key := fmt.Sprintf("test-%d", n)
				value := []byte(fmt.Sprintf("value-%d", n))

				store.Set(ctx, key, value, lib_store.WithTags([]string{fmt.Sprintf("tag-%d", n)}))
			}
		})
	}
}

func BenchmarkRedisGet(b *testing.B) {
	ctx := context.Background()

	store := NewRedis(redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	}))

	key := "test"
	value := []byte("value")

	store.Set(ctx, key, value)

	for k := 0.; k <= 10; k++ {
		n := int(math.Pow(2, k))
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			for i := 0; i < b.N*n; i++ {
				_, _ = store.Get(ctx, key)
			}
		})
	}
}
