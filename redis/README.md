# redis

These ratelimiters are for usage with [Redis](https://redis.io). They're persistent, able to be scaled across multiple instances of your app, and are fully atomic Lua scripts.

These are used in many places in [Fossabot](https://fossabot.com): including but not limited to API ratelimiting, chat abuse detection, follower alert spam limiting, etc.

## Example Usage

The following implements a HTTP server that has a handler ratelimited to 300 requests every 60 seconds.

```go
package main

import (
	"log"
	"net/http"

	"github.com/aidenwallis/go-ratelimiting/redis"
	adapter "github.com/aidenwallis/go-ratelimiting/redis/adapters/go-redis"
	"github.com/aidenwallis/go-write/write"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:6379"})
	ratelimiter := redis.NewLeakyBucket(adapter.NewAdapter(client))

	log.Fatalln((&http.Server{
		Addr: ":8000",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// this endpoint should throttle all requests to it in a leaky bucket called "my-api-endpoint", with a maximum
			// of 300 requests every minute.
			resp, err := ratelimiter.Use(req.Context(), &redis.LeakyBucketOptions{
				KeyPrefix:       "my-api-endpoint",
				MaximumCapacity: 300,
				WindowSeconds:   60,
			})
			if err != nil {
				write.InternalServerError(w).Empty()
				return
			}

			if !resp.Success {
				// request got ratelimited!
				write.TooManyRequests(w).Text("You are being ratelimited.")
				return
			}

			write.Teapot(w).Text("this endpoint is indeed a teapot")
		}),
	}).ListenAndServe())
}
```
