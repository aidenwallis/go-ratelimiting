# local

These ratelimiters only exist within the context of your process and do not share state. If you want a distributed ratelimiter that throttles your clients regardless of restarts, or multiple processes, you should use the Redis ratelimiters instead.

These ratelimiters are thread safe through the use of mutexes, they do not spin up worker goroutines (unless you use `WaitFunc`) and lazily clean themselves up as they're called.

For example, I use `SlidingWindow` for throttling connection writes to Twitch chat.
