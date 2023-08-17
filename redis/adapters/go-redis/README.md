# redigo

An officially supported adapter compatible with [go-redis](https://github.com/redis/go-redis)

## Usage

## Usage

```go
package main

import (
	"github.com/aidenwallis/go-ratelimiting/redis"
	adapter "github.com/aidenwallis/go-ratelimiting/redis/adapters/go-redis"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	client := goredis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	ratelimiter := redis.NewLeakyBucket(adapter.NewAdapter(client))
}
```
