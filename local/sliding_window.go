package local

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrCapacity is returned when the sliding window max size provided is less than or equal to 0
	ErrCapacity = errors.New("capacity must be more than 0")

	// ErrDuration is returned when the sliding window duration provided is less than or equal to 0
	ErrDuration = errors.New("duration must be more than 0")
)

// SlidingWindow provides an interface for the sliding window ratelimiter.
//
// The sliding window ratelimiter is a fixed size window that holds a set of timestamps. When a token is taken, the current time is added to the window.
// The window is constantly cleaned, and evicting old tokens, which allows new ones to be added as the window discards old tokens.
type SlidingWindow interface {
	// Wait will block the goroutine til a ratelimit token is available. You can use context to cancel the ratelimiter.
	Wait(ctx context.Context)

	// WaitFunc is equivalent to Wait except it calls a callback when it's able to accquire a token. Iif you cancel the context, cb is not called. This
	// function does spawn a goroutine per invocation. If you want something more efficient, consider writing your own implementation using TryTakeWithDuration()
	WaitFunc(ctx context.Context, cb func())

	// Size will return how many items are currently sitting in the window
	Size() int

	// Take will attempt to accquire a ratelimit window, it will return a boolean indicating whether it was able to accquire a token or not.
	TryTake() bool

	// Take will attempt to accquire a ratelimit window, it will return a boolean indicating whether it was able to accquire a token or not,
	// and a duration for when you should next try.
	TryTakeWithDuration() (bool, time.Duration)
}

type slidingWindow struct {
	// capacity is the max size of the window
	capacity int
	// duration is how long the token exists in the window for
	duration time.Duration
	// m is the shared mutex to ensure calls are thread safe.
	m sync.Mutex
	// window stores a set of timestamps of when the tokens in the window expire.
	window []time.Time
}

// NewSlidingWindow creates a new sliding window ratelimiter. See the SlidingWindow interface for more info about what this ratelimiter does.
func NewSlidingWindow(capacity int, duration time.Duration) (SlidingWindow, error) {
	if capacity <= 0 {
		return nil, ErrCapacity
	}
	if duration <= 0 {
		return nil, ErrDuration
	}

	return &slidingWindow{
		capacity: capacity,
		duration: duration,
		m:        sync.Mutex{},
		window:   []time.Time{},
	}, nil
}

// clean cleans up the current ratelimit window
func (r *slidingWindow) clean() {
	now := time.Now()
	toRemove := 0

	// find how many keys should be removed from the window.
	for _, ts := range r.window {
		if ts.After(now) {
			// this key hasn't expired yet, stop checking
			break
		}
		toRemove++
	}

	r.window = r.window[toRemove:]
}

// Wait will block the goroutine til a ratelimit token is available. You can use context to cancel the ratelimiter.
func (r *slidingWindow) Wait(ctx context.Context) {
	_ = r.wait(ctx)
}

// wait keeps trying to take a token, while also sleeping the goroutine while it waits for the next attempt. The wait functions just call this
// under the hood.
func (r *slidingWindow) wait(ctx context.Context) bool {
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

// WaitFunc is equivalent to Wait except it calls a callback when it's able to accquire a token. Iif you cancel the context, cb is not called. This
// function does spawn a goroutine per invocation. If you want something more efficient, consider writing your own implementation using TryTakeWithDuration()
func (r *slidingWindow) WaitFunc(ctx context.Context, cb func()) {
	go func(ctx context.Context, cb func()) {
		if r.wait(ctx) {
			cb()
		}
	}(ctx, cb)
}

func (r *slidingWindow) awaitNextToken(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Size will return how many items are currently sitting in the window
func (r *slidingWindow) Size() int {
	r.m.Lock()
	defer r.m.Unlock()
	r.clean()
	return len(r.window)
}

// Take will attempt to accquire a ratelimit window, it will return a boolean indicating whether it was able to accquire a token or not.
func (r *slidingWindow) TryTake() bool {
	resp, _ := r.TryTakeWithDuration()
	return resp
}

// Take will attempt to accquire a ratelimit window, it will return a boolean indicating whether it was able to accquire a token or not,
// and a duration for when you should next try.
func (r *slidingWindow) TryTakeWithDuration() (bool, time.Duration) {
	r.m.Lock()
	defer r.m.Unlock()

	// cleanup any items
	r.clean()

	if len(r.window) >= r.capacity {
		// ratelimit is not available
		return false, time.Until(r.window[0])
	}

	// else add the token
	r.window = append(r.window, time.Now().Add(r.duration))
	return true, 0
}
