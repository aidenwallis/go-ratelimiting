# go-ratelimiting

[![codecov](https://codecov.io/gh/aidenwallis/go-ratelimiting/branch/main/graph/badge.svg?token=I1PYX4TGE9)](https://codecov.io/gh/aidenwallis/go-ratelimiting) [![Go Reference](https://pkg.go.dev/badge/github.com/aidenwallis/go-ratelimiting.svg)](https://pkg.go.dev/github.com/aidenwallis/go-ratelimiting)

Ratelimiting libraries in Go.

These ratelimiters are used in production services for [Fossabot](https://fossabot.com), such as Twitch proxies.

There are two kinds of ratelimiters in this library:

* [**local**](local/README.md): Ratelimiters that are not persistent, and live in-process memory. Useful when you need to throttle a specific function, or some kind of usage within a single container.
* [**redis**](redis/README.md): Ratelimiters that connect to Redis and provide a distributed solution to your ratelimiting problems. Ideal for stateless, distributed applications, such as APIs.
