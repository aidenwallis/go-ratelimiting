package local_test

import (
	"context"
	"testing"
	"time"

	"github.com/aidenwallis/go-ratelimiting/local"
)

func TestLeakyBucket(t *testing.T) {
	t.Parallel() // these tests run in parallel as they involve blocking calls

	t.Run("ratelimits properly", func(t *testing.T) {
		t.Parallel()
		r := local.NewLeakyBucket(10, time.Second*2)

		assertValue(t, 10, r.Size())

		for i := 0; i < 10; i++ {
			assertValue(t, true, r.TryTake())
		}

		assertValue(t, 0, r.Size())

		// should be ratelimited now
		assertValue(t, false, r.TryTake())
	})

	t.Run("blocks goroutine until token is available", func(t *testing.T) {
		t.Parallel()

		r := local.NewLeakyBucket(10, time.Second)

		for i := 0; i < 10; i++ {
			assertValue(t, true, r.TryTake())
		}

		start := time.Now()
		r.Wait(context.Background())

		// kind of hacky but works, timing sleeps isn't great because golangs runtime won't be perfectly accurate, and i didn't want to stub the entire clock
		duration := time.Since(start)
		assertValue(t, 0, r.Size()) // we just used the last token

		// given 10 a second, we should be replenishing 1 token every 100ms, this lets us check that it's roughly correct.
		// if this is wrong, we probably fucked up the tokenRate math
		assertValue(t, true, duration >= time.Millisecond*95 && duration <= time.Millisecond*105)
	})

	t.Run("does not call cb if context is cancelled before token is available", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())

		r := local.NewLeakyBucket(2, time.Millisecond*250)
		for i := 0; i < 2; i++ {
			assertValue(t, true, r.TryTake())
		}

		wasCalled := false
		r.WaitFunc(ctx, func() { wasCalled = true })
		cancel()

		// just in case some weird shit is going on, again, hacky but works
		time.Sleep(time.Millisecond * 500)
		assertValue(t, false, wasCalled)
	})

	t.Run("gives roughly correct take duration", func(t *testing.T) {
		t.Parallel()

		r := local.NewLeakyBucket(2, time.Second)
		for i := 0; i < 2; i++ {
			success, duration := r.TryTakeWithDuration()
			assertValue(t, true, success)
			assertValue(t, 0, duration)
		}

		success, duration := r.TryTakeWithDuration()
		assertValue(t, false, success)
		assertValue(t, true, duration >= time.Millisecond*450 && duration <= time.Millisecond*550)
	})

	t.Run("calls callback in waitFunc", func(t *testing.T) {
		t.Parallel()

		r := local.NewLeakyBucket(2, time.Second)
		for i := 0; i < 2; i++ {
			assertValue(t, true, r.TryTake())
		}

		ch := make(chan struct{}, 1)
		defer close(ch)

		start := time.Now()
		r.WaitFunc(context.Background(), func() {
			ch <- struct{}{}
		})

		<-ch

		duration := time.Since(start)

		// 2 a second means we fill at a constant rate of 500ms, so this checks that it roughly makes sense
		assertValue(t, true, duration >= time.Millisecond*450 && duration <= time.Millisecond*550)
	})
}
