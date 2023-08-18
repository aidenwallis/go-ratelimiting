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

func TestSlidingWindow_Now(t *testing.T) {
	adapter := NewSlidingWindow(nil)
	adapter.nowFunc = nil
	assert.WithinDuration(t, adapter.now(), time.Now(), time.Minute)
}

func TestInspectSlidingWindow(t *testing.T) {
	testCases := map[string]func(*miniredis.Miniredis) adapters.Adapter{
		"go-redis": func(t *miniredis.Miniredis) adapters.Adapter {
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
			limiter := NewSlidingWindow(testCase(miniredis.RunT(t)))
			limiter.nowFunc = func() time.Time { return now }

			{
				// should default to 0
				resp, err := limiter.Inspect(ctx, slidingWindowOptions())
				assert.NoError(t, err)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity, resp.RemainingCapacity)
			}

			{
				// use the bucket
				resp, err := useSlidingWindow(ctx, limiter)
				assert.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-1, resp.RemainingCapacity)
			}

			{
				// inspect again, 1 token should be missing
				resp, err := limiter.Inspect(ctx, slidingWindowOptions())
				assert.NoError(t, err)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-1, resp.RemainingCapacity)
			}
		})
	}
}

func TestInspectSlidingWindow_Errors(t *testing.T) {
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
			errorMessage: "expecting int64 but got string",
			mockAdapter: &mockAdapter{
				returnValue: "foo",
			},
		},
	}

	for name, testCase := range testCases {
		testCase := testCase

		t.Run(name, func(t *testing.T) {
			out, err := NewSlidingWindow(testCase.mockAdapter).Inspect(context.Background(), slidingWindowOptions())
			assert.Nil(t, out)
			assert.EqualError(t, err, testCase.errorMessage)
		})
	}
}

func TestUseSlidingWindow(t *testing.T) {
	testCases := map[string]func(*miniredis.Miniredis) adapters.Adapter{
		"go-redis": func(t *miniredis.Miniredis) adapters.Adapter {
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
			limiter := NewSlidingWindow(testCase(miniredis.RunT(t)))
			limiter.nowFunc = func() time.Time { return now }

			{
				resp, err := useSlidingWindow(ctx, limiter)
				assert.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-1, resp.RemainingCapacity)
			}

			// move forward 3 seconds
			limiter.nowFunc = func() time.Time { return now.Add(time.Second * 3) }

			{
				resp, err := useSlidingWindow(ctx, limiter)
				assert.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-2, resp.RemainingCapacity, "tokens shouldn't have expired yet")
			}

			// move forward 60 seconds
			limiter.nowFunc = func() time.Time { return now.Add(time.Second * 60) }

			{
				resp, err := useSlidingWindow(ctx, limiter)
				assert.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-2, resp.RemainingCapacity, "one token should've expired, so including this request, 2 should be used")
			}

			// move forward 120 seconds
			limiter.nowFunc = func() time.Time { return now.Add(time.Second * 120) }

			{
				resp, err := useSlidingWindow(ctx, limiter)
				assert.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Equal(t, leakyBucketOptions().MaximumCapacity-1, resp.RemainingCapacity, "all tokens should've expired by now, so only this one is left")
			}
		})
	}
}

func TestUseSlidingWindow_Errors(t *testing.T) {
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
			out, err := useSlidingWindow(context.Background(), NewSlidingWindow(testCase.mockAdapter))
			assert.Nil(t, out)
			assert.EqualError(t, err, testCase.errorMessage)
		})
	}
}

func TestParseSlidingWindowResponse_Errors(t *testing.T) {
	testCases := map[string]struct {
		errorMessage string
		in           interface{}
	}{
		"invalid type": {
			errorMessage: "expected []interface{} but got string",
			in:           "foo",
		},
		"invalid length": {
			errorMessage: "expected 2 args but got 3",
			in:           []interface{}{int64(1), int64(2), int64(3)},
		},
	}

	for name, testCase := range testCases {
		testCase := testCase

		t.Run(name, func(t *testing.T) {
			out, err := parseSlidingWindowResponse(testCase.in)
			assert.Nil(t, out)
			assert.EqualError(t, err, testCase.errorMessage)
		})
	}
}

// slidingWindowOptions provides quick sane defaults for testing sliding windows
func slidingWindowOptions() *SlidingWindowOptions {
	return &SlidingWindowOptions{
		Key:             "test-bucket",
		MaximumCapacity: 60,
		Window:          time.Minute,
	}
}

// useSlidingWindow is a helper to test your sliding window with some predefined options
func useSlidingWindow(ctx context.Context, limiter SlidingWindow) (*UseSlidingWindowResponse, error) {
	return limiter.Use(ctx, slidingWindowOptions())
}
