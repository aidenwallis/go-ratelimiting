package adapters

import "context"

// Adapter provides a generic interface that's compatible with various Go redis libraries.
//
// This package ships with native support for [go-redis] and [redigo], see [github.com/aidenwallis/go-ratelimiting/redis/adapters/go-redis]
// and [github.com/aidenwallis/redis/adapters/redigo].
//
// Alternatively, if you ship your own Redis implementation, you can build your own wrapper compatible with this interface to consume this
// package.
//
// [go-redis]: https://github.com/redis/go-redis
// [redigo]: https://github.com/gomodule/redigo
type Adapter interface {
	// Eval adds support for the redis EVAL command
	//
	// See https://redis.io/commands/eval
	Eval(ctx context.Context, script string, keys []string, args []interface{}) (output interface{}, err error)
}
