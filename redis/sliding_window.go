package redis

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
)

// SlidingWindow provides an interface for the redis sliding window ratelimiter, compatible with SlidingWindowImpl
//
// The sliding window ratelimiter is a fixed size window that holds a set of timestamps. When a token is taken, the current time is added to the window.
// The window is constantly cleaned, and evicting old tokens, which allows new ones to be added as the window discards old tokens.
type SlidingWindow interface {
	// Inspect atomically inspects the sliding window and returns the capacity available. It does not take any tokens.
	Inspect(ctx context.Context, bucket *SlidingWindowOptions) (*InspectSlidingWindowResponse, error)

	// Use atomically attempts to use the sliding window. Sliding window ratelimiters always take 1 token at a time, as the key is inferred
	// from when it would expire in nanoseconds.
	Use(ctx context.Context, bucket *SlidingWindowOptions) (*UseSlidingWindowResponse, error)
}

var _ SlidingWindow = (*SlidingWindowImpl)(nil)

// SlidingWindowImpl implements a sliding window ratelimiter for Redis using Lua. This struct is compatible with the SlidingWindow interface.
//
// Refer to the SlidingWindow interface for more information about this ratelimiter.
type SlidingWindowImpl struct {
	// Adapter defines the Redis adapter
	Adapter adapters.Adapter

	// nowFunc is a private helper used to mock out time changes in unit testing
	//
	// if this is not defined, it falls back to time.Now()
	nowFunc func() time.Time
}

// SlidingWindowOptions defines the options available to a sliding window bucket.
type SlidingWindowOptions struct {
	// Key defines the Redis key used for this sliding window ratelimiter
	Key string

	// MaximumCapacity defines the max size of the sliding window, no more tokens than this may be stored in the sliding
	// window at any time.
	MaximumCapacity int

	// Window defines the size of the sliding window, resolution is available up to nanoseconds.
	Window time.Duration
}

// NewSlidingWindow creates a new sliding window instance
func NewSlidingWindow(adapter adapters.Adapter) *SlidingWindowImpl {
	return &SlidingWindowImpl{
		Adapter: adapter,
		nowFunc: time.Now,
	}
}

func (r *SlidingWindowImpl) now() time.Time {
	if r.nowFunc == nil {
		return time.Now()
	}
	return r.nowFunc()
}

// InspectSlidingWindowResponse defines the response parameters for SlidingWindow.Inspect()
type InspectSlidingWindowResponse struct {
	// RemainingCapacity defines the remaining amount of capacity left in the bucket
	RemainingCapacity int
}

// Inspect inspects the current state of the sliding window bucket
func (r *SlidingWindowImpl) Inspect(ctx context.Context, bucket *SlidingWindowOptions) (*InspectSlidingWindowResponse, error) {
	const script = `
local key = KEYS[1]
local now = ARGV[1]

redis.call("zremrangebyscore", key, "-inf", now) -- clear expired tokens

local tokens = tonumber(redis.call("zcard", key))
if (tokens == nil) then
	tokens = 0
end

return tokens
`

	resp, err := r.Adapter.Eval(ctx, script, []string{bucket.Key}, []interface{}{r.now().UnixNano()})
	if err != nil {
		return nil, fmt.Errorf("failed to query redis adapter: %w", err)
	}

	tokens, ok := resp.(int64)
	if !ok {
		return nil, fmt.Errorf("expecting int64 but got %T", resp)
	}

	remaining := 0
	if v := bucket.MaximumCapacity - int(tokens); v > 0 {
		remaining = v
	}

	return &InspectSlidingWindowResponse{
		RemainingCapacity: remaining,
	}, nil
}

// UseSlidingWindowResponse defines the response parameters for SlidingWindow.Use()
type UseSlidingWindowResponse struct {
	// Success defines whether the sliding window was successfully used
	Success bool

	// RemainingCapacity defines the remaining amount of capacity left in the bucket
	RemainingCapacity int
}

// Use atomically attempts to use the sliding window.
func (r *SlidingWindowImpl) Use(ctx context.Context, bucket *SlidingWindowOptions) (*UseSlidingWindowResponse, error) {
	const script = `
local key = KEYS[1]
local now = ARGV[1]
local expiresAt = ARGV[2]
local window = ARGV[3]
local max = tonumber(ARGV[4])

redis.call("zremrangebyscore", key, "-inf", now) -- clear expired tokens

local tokens = tonumber(redis.call("zcard", key))
if (tokens == nil) then
	tokens = 0 -- default tokens to 0
end

local success = 0

if (tokens < max) then
	-- room available: add a token, bump ttl, and include newly added token in count
	redis.call("zadd", key, expiresAt, expiresAt)
	redis.call("expire", key, window)
	success = 1
	tokens = tokens + 1
end

return {success, tokens}
	`

	now := r.now()
	current := now.UnixNano()
	expiresAt := now.Add(bucket.Window).UnixNano()
	windowTTL := int(math.Ceil(bucket.Window.Seconds()))

	resp, err := r.Adapter.Eval(ctx, script, []string{bucket.Key}, []interface{}{
		current, expiresAt, windowTTL, bucket.MaximumCapacity,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query redis adapter: %w", err)
	}

	output, err := parseSlidingWindowResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("parsing redis response: %w", err)
	}

	remaining := 0
	if v := bucket.MaximumCapacity - output.tokens; v > remaining {
		remaining = v
	}

	return &UseSlidingWindowResponse{
		Success:           output.success,
		RemainingCapacity: remaining,
	}, nil
}

type slidingWindowOutput struct {
	success bool
	tokens  int
}

func parseSlidingWindowResponse(v interface{}) (*slidingWindowOutput, error) {
	ints, err := parseRedisInt64Slice(v)
	if err != nil {
		return nil, err
	}

	if len(ints) != 2 {
		return nil, fmt.Errorf("expected 2 args but got %d", len(ints))
	}

	return &slidingWindowOutput{
		success: ints[0] == 1,
		tokens:  int(ints[1]),
	}, nil
}
