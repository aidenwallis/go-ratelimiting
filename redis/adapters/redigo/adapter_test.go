package redigo_test

import (
	"testing"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters/internal/adaptertests"
	"github.com/aidenwallis/go-ratelimiting/redis/adapters/redigo"
	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/assert"
)

func TestAdapter(t *testing.T) {
	mr := miniredis.RunT(t)

	conn, err := redis.Dial("tcp", mr.Addr())
	assert.NoError(t, err)

	adaptertests.BattletestAdapter(t, mr, redigo.NewAdapter(conn))
}
