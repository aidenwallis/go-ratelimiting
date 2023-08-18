package redis

import "fmt"

func parseRedisInt64Slice(v interface{}) ([]int64, error) {
	args, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected []interface{} but got %T", v)
	}

	out := make([]int64, len(args))
	for i, arg := range args {
		value, ok := arg.(int64)
		if !ok {
			return nil, fmt.Errorf("expected int64 in args[%d] but got %T", i, arg)
		}

		out[i] = value
	}

	return out, nil
}
