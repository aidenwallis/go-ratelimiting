package redis

import (
	"context"
	"testing"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
	"github.com/stretchr/testify/assert"
)

type mockAdapter struct {
	called      bool
	returnValue interface{}
	returnError error
}

var _ adapters.Adapter = (*mockAdapter)(nil)

func (a *mockAdapter) Eval(_ context.Context, _ string, _ []string, _ []interface{}) (interface{}, error) {
	a.called = true
	return a.returnValue, a.returnError
}

func TestParseRedisInt64Slice(t *testing.T) {
	t.Run("errors", func(t *testing.T) {
		testCases := map[string]struct {
			errorMessage string
			in           interface{}
		}{
			"invalid type": {
				errorMessage: "expected []interface{} but got string",
				in:           "foo",
			},
			"invalid value in slice": {
				errorMessage: "expected int64 in args[1] but got string",
				in:           []interface{}{int64(1), "two", int64(3)},
			},
		}

		for name, testCase := range testCases {
			testCase := testCase

			t.Run(name, func(t *testing.T) {
				out, err := parseRedisInt64Slice(testCase.in)
				assert.Nil(t, out)
				assert.EqualError(t, err, testCase.errorMessage)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		out, err := parseRedisInt64Slice([]interface{}{int64(1), int64(2), int64(3)})
		assert.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3}, out)
	})
}
