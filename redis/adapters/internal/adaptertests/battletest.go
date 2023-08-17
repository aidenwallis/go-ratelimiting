package adaptertests

import (
	"context"
	"testing"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
)

// BattletestAdapter is a helper to quickly test that an adapter is functioning correctly
func BattletestAdapter(t *testing.T, mr *miniredis.Miniredis, adapter adapters.Adapter) {
	// Script is a test script used for testing that adapters are working properly
	const Script = `
redis.call("set", tostring(KEYS[1]), tostring(ARGV[1]))
return 1
	`

	key := "foo"
	value := "value"

	out, err := adapter.Eval(context.Background(), Script, []string{key}, []interface{}{value})
	assert.NoError(t, err)

	assert.EqualValues(t, 1, out.(int64))

	getValue, err := mr.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, getValue)
}
