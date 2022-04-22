package local_test

import (
	"context"
	"testing"
	"time"

	"github.com/aidenwallis/go-ratelimiting/local"
)

func TestSlidingWindow(t *testing.T) {
	t.Parallel() // these tests run in parallel as they involve blocking calls

	t.Run("validates arguments correctly", func(t *testing.T) {
		t.Parallel()

		_, err := local.NewSlidingWindow(0, time.Second*10)
		assertValue(t, local.ErrCapacity.Error(), err.Error())

		_, err = local.NewSlidingWindow(10, 0)
		assertValue(t, local.ErrDuration.Error(), err.Error())
	})

	t.Run("ratelimits properly", func(t *testing.T) {
		t.Parallel()
		r, err := local.NewSlidingWindow(10, time.Second*2)
		assertNoError(t, err)

		assertValue(t, 0, r.Size())

		for i := 0; i < 10; i++ {
			assertValue(t, true, r.TryTake())
		}

		assertValue(t, 10, r.Size())

		// should be ratelimited now
		assertValue(t, false, r.TryTake())
	})

	t.Run("blocks goroutine until token is available", func(t *testing.T) {
		t.Parallel()

		r, _ := local.NewSlidingWindow(10, time.Second)

		for i := 0; i < 10; i++ {
			assertValue(t, true, r.TryTake())
		}

		start := time.Now()
		r.Wait(context.Background())

		// kind of hacky but works, timing sleeps isn't great because golangs runtime won't be perfectly accurate, and i didn't want to stub the entire clock
		duration := time.Since(start)
		assertValue(t, 1, r.Size())
		assertValue(t, true, duration >= time.Millisecond*950 && duration <= time.Millisecond*1050)
	})

	t.Run("does not call cb if context is cancelled before token is available", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())

		r, _ := local.NewSlidingWindow(2, time.Millisecond*250)
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

		r, _ := local.NewSlidingWindow(2, time.Second)
		for i := 0; i < 2; i++ {
			success, duration := r.TryTakeWithDuration()
			assertValue(t, true, success)
			assertValue(t, 0, duration)
		}

		success, duration := r.TryTakeWithDuration()
		assertValue(t, false, success)
		assertValue(t, true, duration >= time.Millisecond*950 && duration <= time.Millisecond*1050)
	})

	t.Run("calls callback in waitFunc", func(t *testing.T) {
		t.Parallel()

		r, _ := local.NewSlidingWindow(2, time.Second)
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
		assertValue(t, true, duration >= time.Millisecond*950 && duration <= time.Millisecond*1050)
	})
}

func assertValue[T comparable](t *testing.T, expected, actualValue T) {
	if expected != actualValue {
		t.Errorf("expected value %v but got %v", expected, actualValue)
	}
}

func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Error("got error when should be nil: %w", err)
	}
}
