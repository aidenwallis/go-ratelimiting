package local

import (
	"context"
	"math"
	"sync"
	"time"
)

// LeakyBucket is a ratelimiter that fills a given bucket at a constant rate you define (calculated based on your window duration, and the max tokens)
// that may exist in the window at any given time.
//
// Leaky buckets have the advantage of being able to burst up to the max tokens you define, and then slowly leak out tokens at a constant rate. This makes
// it a good fit for situations where you want caller buckets to slowly fill if they decide to burst your service, whereas a sliding window ratelimiter will
// free all tokens at once.
//
// Leaky buckets slowly fill your window over time, and will not fill above the size of the window. For example, if you allow 10 tokens per a window of 1 second,
// your bucket fills at a fixed rate of 100ms.
//
// See: https://en.wikipedia.org/wiki/Leaky_bucket
type LeakyBucket interface {
	// Wait will block the goroutine til a ratelimit token is available. You can use context to cancel the ratelimiter.
	Wait(ctx context.Context)

	// WaitFunc is equivalent to Wait except it calls a callback when it's able to accquire a token. Iif you cancel the context, cb is not called. This
	// function does spawn a goroutine per invocation. If you want something more efficient, consider writing your own implementation using TryTakeWithDuration()
	WaitFunc(ctx context.Context, cb func())

	// Size will return how many tokens are currently available
	Size() int

	// Take will attempt to accquire a token, it will return a boolean indicating whether it was able to accquire a token or not.
	TryTake() bool

	// Take will attempt to accquire a token, it will return a boolean indicating whether it was able to accquire a token or not,
	// and a duration for when you should next try.
	TryTakeWithDuration() (bool, time.Duration)
}

type leakyBucket struct {
	max      int
	tokens   int
	rate     time.Duration
	lastFill time.Time
	m        sync.Mutex
}

// NewLeakyBucket creates a new leaky bucket ratelimiter. See the LeakyBucket interface for more info about what this ratelimiter does.
func NewLeakyBucket(tokensPerWindow int, window time.Duration) LeakyBucket {
	tokenRate := window / time.Duration(tokensPerWindow)

	return &leakyBucket{
		tokens:   tokensPerWindow,
		lastFill: time.Now().UTC(),
		max:      tokensPerWindow,
		rate:     tokenRate,
	}
}

// TryTakeWithDuration will attempt to accquire a ratelimit window, it will return a boolean indicating whether it was able to accquire a token or not,
// and a duration for when you should next try.
func (r *leakyBucket) TryTakeWithDuration() (bool, time.Duration) {
	r.m.Lock()
	defer r.m.Unlock()

	r.unsafeFill()

	if r.tokens < 1 {
		// there isn't at least 1 oken, so nothing is available
		return false, time.Until(r.lastFill.Add(r.rate))
	}

	// take a token if there is one available
	r.tokens--

	return true, 0
}

// Take will attempt to accquire a ratelimit window, it will return a boolean indicating whether it was able to accquire a token or not.
func (r *leakyBucket) TryTake() bool {
	resp, _ := r.TryTakeWithDuration()
	return resp
}

// Wait will block the goroutine til a ratelimit token is available. You can use context to cancel the ratelimiter.
func (r *leakyBucket) Wait(ctx context.Context) {
	_ = r.wait(ctx)
}

// wait keeps trying to take a token, while also sleeping the goroutine while it waits for the next attempt. The wait functions just call this
// under the hood.
func (r *leakyBucket) wait(ctx context.Context) bool {
	for {
		available, duration := r.TryTakeWithDuration()
		if available {
			return true
		}
		if !r.awaitNextToken(ctx, duration) {
			return false
		}
	}
}

// Size will return how many tokens are currently available
func (r *leakyBucket) Size() int {
	r.m.Lock()
	defer r.m.Unlock()
	r.unsafeFill()
	return r.tokens
}

// WaitFunc is equivalent to Wait except it calls a callback when it's able to accquire a token. Iif you cancel the context, cb is not called. This
// function does spawn a goroutine per invocation. If you want something more efficient, consider writing your own implementation using TryTakeWithDuration()
func (r *leakyBucket) WaitFunc(ctx context.Context, cb func()) {
	go func(ctx context.Context, cb func()) {
		if r.wait(ctx) {
			cb()
		}
	}(ctx, cb)
}

func (r *leakyBucket) awaitNextToken(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// unsafeFill attempts to fill the leaky bucket with tokens, but is not thread safe.
//
// Ensure you have locked the mutex outside of this function before calling it.
func (r *leakyBucket) unsafeFill() {
	if r.tokens >= r.max {
		// bucket is already full
		return
	}

	tokensToFill := int(time.Since(r.lastFill) / r.rate)
	r.tokens = int(math.Min(float64(r.tokens+tokensToFill), float64(r.max)))
	r.lastFill = time.Now().UTC()
}
