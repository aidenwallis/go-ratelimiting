package redis

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
)

// LeakyBucket defines an interface compatible with LeakyBucketImpl
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
	// Use atomically attempts to use the leaky bucket. Use takeAmount to set how many tokens should be attempted to be removed
	// from the bucket: they are atomic, either all tokens are taken, or the ratelimit is unsuccessful.
	Use(ctx context.Context, bucket *LeakyBucketOptions, takeAmount int) (*UseLeakyBucketResponse, error)
}

var _ LeakyBucket = (*LeakyBucketImpl)(nil)

// LeakyBucketOptions defines the options available to LeakyBucket ratelimiters
type LeakyBucketOptions struct {
	// KeyPrefix is the bucket key name in Redis.
	//
	// Note that this ratelimiter will create two keys in Redis, and suffix them with :last_fill and :tokens.
	KeyPrefix string

	// MaximumCapacity defines the maximum number of tokens in the leaky bucket. If a bucket has expired or otherwise doesn't exist,
	// the bucket is set to this size, it also ensures the bucket can never contain more than this number of tokens at any time.
	//
	// Note that if you decrease the number of tokens in an existing bucket, that bucket is automatically reduced to the new max size,
	// however, if you increase the maximum capacity of the bucket, it will refill faster, but not immediately be placed to the new, higher
	// capacity.
	MaximumCapacity int

	// WindowSeconds defines the maximum amount of time it takes to refill the bucket, the refill rate of the bucket is calculated using
	// maximumCapacity/windowSeconds, in other words, if your capacity was 60 tokens, and the window was 1 minute, you would refill at a constant
	// rate of 1 token per second.
	//
	// Windows have a maximum resolution of 1 second.
	WindowSeconds int
}

// UseLeakyBucketResponse defines the response parameters for LeakyBucket.Use()
type UseLeakyBucketResponse struct {
	// Success is true when we were successfully able to take tokens from the bucket.
	Success bool

	// RemainingTokens defines hwo many tokens are left in the bucket
	RemainingTokens int

	// ResetAt is the time at which the bucket will be fully refilled
	ResetAt time.Time
}

// LeakyBucketImpl implements a leaky bucket ratelimiter in Redis with Lua. This struct is compatible with the LeakyBucket interface
//
// See the LeakyBucket interface for more information about leaky bucket ratelimiters.
type LeakyBucketImpl struct {
	// Adapter defines the Redis adapter
	Adapter adapters.Adapter

	// nowFunc is a private helper used to mock out time changes in unit testing
	nowFunc func() time.Time
}

// NewLeakyBucket creates a new leaky bucket instance
func NewLeakyBucket(adapter adapters.Adapter) *LeakyBucketImpl {
	return &LeakyBucketImpl{
		Adapter: adapter,
		nowFunc: time.Now,
	}
}

func (r *LeakyBucketImpl) now() time.Time {
	if r.nowFunc == nil {
		return time.Now()
	}
	return r.nowFunc()
}

// Use atomically attempts to use the leaky bucket. Use takeAmount to set how many tokens should be attempted to be removed
// from the bucket: they are atomic, either all tokens are taken, or the ratelimit is unsuccessful.
func (r *LeakyBucketImpl) Use(ctx context.Context, bucket *LeakyBucketOptions, takeAmount int) (*UseLeakyBucketResponse, error) {
	const script = `
local tokensKey = KEYS[1]
local lastFillKey = KEYS[2]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local take = tonumber(ARGV[4])
local windowSeconds = ARGV[5]

local tokens = tonumber(redis.call("get", tokensKey))
local lastFilled = tonumber(redis.call("get", lastFillKey))

if (tokens == nil) then
	tokens = 0 -- default empty buckets to 0
end

if (tokens > capacity) then
	tokens = capacity -- shrink buckets if the capacity is reduced
end

if (lastFilled == nil) then
	lastFilled = 0
end

if (tokens < capacity) then
	local tokensToFill = math.floor((now - lastFilled) * rate)
	if (tokensToFill > 0) then
		tokens = math.min(capacity, tokens + tokensToFill)
		lastFilled = now
	end
end

local success = 0

if (tokens >= take) then
	tokens = tokens - take
	success = 1
end

redis.call("set", tokensKey, tostring(tokens), "EX", windowSeconds)
redis.call("set", lastFillKey, tostring(lastFilled), "EX", windowSeconds)

return {success, tokens, lastFilled}
	`

	refillRate := getRefillRate(bucket.MaximumCapacity, bucket.WindowSeconds)
	now := r.now().UTC().Unix()

	tokensKey := bucket.KeyPrefix + "::tokens"
	lastFillKey := bucket.KeyPrefix + "::last_fill"

	resp, err := r.Adapter.Eval(ctx, script, []string{tokensKey, lastFillKey}, []interface{}{bucket.MaximumCapacity, refillRate, now, takeAmount, bucket.WindowSeconds})
	if err != nil {
		return nil, fmt.Errorf("failed to query redis adapter: %w", err)
	}

	output, err := parseLeakyBucketResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("parsing redis response: %w", err)
	}

	return &UseLeakyBucketResponse{
		Success:         output.success,
		RemainingTokens: output.remaining,
		ResetAt:         calculateLeakyBucketFillTime(output.lastFilled, output.remaining, bucket.MaximumCapacity, bucket.WindowSeconds),
	}, nil
}

func calculateLeakyBucketFillTime(lastFillUnix, currentTokens, maxCapacity, windowSeconds int) time.Time {
	resetAt := lastFillUnix // if delta is 0 (thus, all tokens are filled), then the bucket is already reset
	if delta := maxCapacity - currentTokens; delta > 0 {
		// determine how many tokens we add per second, we'll need to use that to calculate how long it'll take us to fill back up to max
		rate := getRefillRate(maxCapacity, windowSeconds)

		// calculate how long many seconds it takes to fill at the token rate we have, but if the full window is smaller, use that, as the
		// bucket must be full by the time the window hits.
		secondsTillRefill := windowSeconds
		if calculatedSeconds := int(math.Ceil(float64(delta) / rate)); calculatedSeconds < secondsTillRefill {
			secondsTillRefill = calculatedSeconds
		}

		resetAt += secondsTillRefill
	}

	return time.Unix(int64(resetAt), 0)
}

func getRefillRate(maxCapacity, windowSeconds int) float64 {
	return float64(maxCapacity) / float64(windowSeconds)
}

type leakyBucketOutput struct {
	success    bool
	remaining  int
	lastFilled int
}

func parseLeakyBucketResponse(v interface{}) (*leakyBucketOutput, error) {
	args, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected []interface{} but got %T", v)
	}

	if len(args) != 3 {
		return nil, fmt.Errorf("expected 3 args but got %d", len(args))
	}

	argInts := make([]int64, len(args))
	for i, argValue := range args {
		intValue, ok := argValue.(int64)
		if !ok {
			return nil, fmt.Errorf("expected int64 in arg[%d] but got %T", i, argValue)
		}

		argInts[i] = intValue
	}

	return &leakyBucketOutput{
		success:    argInts[0] == 1,
		remaining:  int(argInts[1]),
		lastFilled: int(argInts[2]),
	}, nil
}
