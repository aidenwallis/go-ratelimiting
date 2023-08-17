package goredis

import (
	"context"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
	"github.com/redis/go-redis/v9"
)

// Adapter is a [go-redis] implementation compatible with [github.com/aidenwallis/go-ratelimiting/redis/adapters]
//
// [go-redis]: https://github.com/redis/go-redis
type Adapter struct {
	Client *redis.Client
}

var _ adapters.Adapter = (*Adapter)(nil)

// NewAdapter creates a new adapter using the [go-redis] client.
//
// [go-redis]: https://github.com/redis/go-redis
func NewAdapter(client *redis.Client) *Adapter {
	return &Adapter{
		Client: client,
	}
}

// Eval defines adapter compatibility for the redis EVAL command
func (a *Adapter) Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error) {
	return a.Client.Eval(ctx, script, keys, args...).Result()
}
