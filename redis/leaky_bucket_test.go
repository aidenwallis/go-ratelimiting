package redis

import (
	"context"
	"testing"
	"time"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
	goredisadapter "github.com/aidenwallis/go-ratelimiting/redis/adapters/go-redis"
	redigoadapter "github.com/aidenwallis/go-ratelimiting/redis/adapters/redigo"
	"github.com/alicebob/miniredis/v2"
	redigo "github.com/gomodule/redigo/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestUseLeakyBucket(t *testing.T) {
	t.Parallel()

	testCases := map[string]func(*miniredis.Miniredis) adapters.Adapter{
		"goredis": func(t *miniredis.Miniredis) adapters.Adapter {
			return goredisadapter.NewAdapter(goredis.NewClient(&goredis.Options{Addr: t.Addr()}))
		},
		"redigo": func(t *miniredis.Miniredis) adapters.Adapter {
			conn, err := redigo.Dial("tcp", t.Addr())
			if err != nil {
				panic(err)
			}
			return redigoadapter.NewAdapter(conn)
		},
	}

	for name, testCase := range testCases {
		testCase := testCase

		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			now := time.Now().UTC()
			limiter := NewLeakyBucket(testCase(miniredis.RunT(t)))
			limiter.nowFunc = func() time.Time { return now }

			{
				resp, err := useLeakyBucket(ctx, limiter)
				assert.NoError(t, err)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-1, resp.RemainingTokens)
				assert.Equal(t, now.Add(time.Second).Unix(), resp.ResetAt.Unix())
			}

			{
				resp, err := useLeakyBucket(ctx, limiter)
				assert.NoError(t, err)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-2, resp.RemainingTokens)
				assert.Equal(t, now.Add(time.Second*2).Unix(), resp.ResetAt.Unix())
			}

			// move forward 3 seconds
			limiter.nowFunc = func() time.Time { return now.Add(time.Second * 3) }

			{
				resp, err := useLeakyBucket(ctx, limiter)
				assert.NoError(t, err)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-1, resp.RemainingTokens)
				assert.Equal(t, now.Add(time.Second*4).Unix(), resp.ResetAt.Unix())
			}
		})
	}
}

func TestLeakyBucket_Now(t *testing.T) {
	adapter := NewLeakyBucket(nil)
	adapter.nowFunc = nil
	assert.WithinDuration(t, adapter.now(), time.Now(), time.Minute)
}

func TestUseLeakyBucket_Errors(t *testing.T) {
	testCases := map[string]struct {
		errorMessage string
		mockAdapter  adapters.Adapter
	}{
		"redis error": {
			errorMessage: "failed to query redis adapter: " + assert.AnError.Error(),
			mockAdapter: &mockAdapter{
				returnError: assert.AnError,
			},
		},
		"parsing error": {
			errorMessage: "parsing redis response: expected []interface{} but got string",
			mockAdapter: &mockAdapter{
				returnValue: "foo",
			},
		},
	}

	for name, testCase := range testCases {
		testCase := testCase

		t.Run(name, func(t *testing.T) {
			out, err := useLeakyBucket(context.Background(), NewLeakyBucket(testCase.mockAdapter))
			assert.Nil(t, out)
			assert.EqualError(t, err, testCase.errorMessage)
		})
	}
}

func TestRefillRate(t *testing.T) {
	assert.EqualValues(t, 1.5, getRefillRate(90, 60))
	assert.EqualValues(t, 1, getRefillRate(60, 60))
	assert.EqualValues(t, 5, getRefillRate(300, 60))
}

func TestParseLeakyBucketResponse_Errors(t *testing.T) {
	testCases := map[string]struct {
		errorMessage string
		in           interface{}
	}{
		"invalid type": {
			errorMessage: "expected []interface{} but got string",
			in:           "foo",
		},
		"invalid length": {
			errorMessage: "expected 3 args but got 2",
			in:           []interface{}{1, 2},
		},
		"invalid item type": {
			errorMessage: "expected int64 in arg[1] but got float64",
			in:           []interface{}{int64(1), float64(2), "three"},
		},
	}

	for name, testCase := range testCases {
		testCase := testCase

		t.Run(name, func(t *testing.T) {
			out, err := parseLeakyBucketResponse(testCase.in)
			assert.Nil(t, out)
			assert.EqualError(t, err, testCase.errorMessage)
		})
	}
}

// leakyBucketOptions provides quick sane defaults for testing leaky bucket options
func leakyBucketOptions() *LeakyBucketOptions {
	return &LeakyBucketOptions{
		KeyPrefix:       "test-bucket",
		MaximumCapacity: 60,
		WindowSeconds:   60,
	}
}

// useLeakyBucket is a helper to test your leaky bucket with some predefined options
func useLeakyBucket(ctx context.Context, limiter LeakyBucket) (*UseLeakyBucketResponse, error) {
	return limiter.Use(ctx, leakyBucketOptions(), 1)
}
