package redis

import (
	"context"

	"github.com/aidenwallis/go-ratelimiting/redis/adapters"
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
