# redigo

An officially supported adapter compatible with [redigo](https://github.com/gomodule/redigo)

## Usage

```go
package main

import (
	"log"

	"github.com/aidenwallis/go-ratelimiting/redis"
	"github.com/aidenwallis/go-ratelimiting/redis/adapters/redigo"
	redigo "github.com/gomodule/redigo/redis"
)

func main() {
	conn, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		log.Fatalf("failed to connect to redis: %w", err)
	}

	ratelimiter := redis.NewLeakyBucket(redigo.NewAdapter(conn))
}

```
