package goredis_test

import (
	"testing"

	goredis "github.com/aidenwallis/go-ratelimiting/redis/adapters/go-redis"
	"github.com/aidenwallis/go-ratelimiting/redis/adapters/internal/adaptertests"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestAdapter(t *testing.T) {
	mr := miniredis.RunT(t)
	adaptertests.BattletestAdapter(t, mr, goredis.NewAdapter(redis.NewClient(&redis.Options{Addr: mr.Addr()})))
}
