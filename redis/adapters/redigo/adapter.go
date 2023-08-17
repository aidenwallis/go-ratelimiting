package redigo

import (
	"context"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
	"github.com/gomodule/redigo/redis"
)

// Adapter is a [redigo] implementation compatible with [github.com/aidenwallis/go-ratelimiting/redis/adapters]
//
// [redigo]: https://github.com/gomodule/redigo
type Adapter struct {
	Conn redis.Conn
}

var _ adapters.Adapter = (*Adapter)(nil)

// NewAdapter creates a new adapter using the [redigo] client.
//
// [redigo]: https://github.com/gomodule/redigo
func NewAdapter(conn redis.Conn) *Adapter {
	return &Adapter{Conn: conn}
}

// Eval defines adapter compatibility for the redis EVAL command
func (a *Adapter) Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error) {
	return redis.DoContext(a.Conn, ctx, "EVAL", buildEvalArgs(script, keys, args...)...)
}

func buildEvalArgs(script string, keys []string, args ...interface{}) []interface{} {
	out := make([]interface{}, 0, 2+len(keys)+len(args))
	out = append(out, script, len(keys))
	for _, v := range keys {
		out = append(out, v)
	}
	return append(out, args...)
}
