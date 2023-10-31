package util

import (
	"context"
	"sync/atomic"
	"time"
)

type HasTimeout interface{ Timeout() time.Duration }

func GoEach(ctx context.Context, f ...func(context.Context)) {
	GoItems(ctx, f, func(ctx context.Context, f func(context.Context)) { f(ctx) }, 0, 0)
}

// GoItems is like a `sync.WaitGroup` when `maxConcurrentOps`
// is 0, but a semaphore with a `maxConcurrentOps` greater than 1.
// If `T` implements `HasTimeout` and `opTimeout` is 0, only `T.Timeout()` is used.
// If `opTimeout` is greater than `0`, this indicates that `T.Timeout()` is to be ignored for this particular run of `op`s.
func GoItems[T any](ctx context.Context, workSet []T, op func(context.Context, T), maxConcurrentOps int, opTimeout time.Duration) {
	if len(workSet) == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callOp := func(ctx context.Context, item T) {
		timeout := opTimeout
		if customTimeout, _ := (any(item)).(HasTimeout); timeout == 0 && customTimeout != nil {
			timeout = customTimeout.Timeout()
		}
		DoTimeout(ctx, timeout, func(ctx context.Context) {
			op(ctx, item)
		})
	}

	if maxConcurrentOps == 1 || len(workSet) == 1 {
		for _, item := range workSet {
			callOp(ctx, item)
		}
		return
	}

	allAtOnce, sema := (maxConcurrentOps == 0), make(chan struct{}, maxConcurrentOps)
	var numDone atomic.Int64
	for _, item := range workSet {
		go func(item T) {
			if allAtOnce {
				<-sema
			}
			callOp(ctx, item)
			numDone.Add(1)
			if !allAtOnce {
				<-sema
			}
		}(item)
		sema <- struct{}{}
	}
	for n := int64(len(workSet)); numDone.Load() < n; { // wait for all
		time.Sleep(time.Millisecond) // lets cpu breathe just a bit... without unduly delaying things
	}
}

func DoTimeout(ctx context.Context, timeout time.Duration, op func(context.Context)) {
	if timeout != 0 {
		ctxTimeout, done := context.WithTimeout(ctx, timeout)
		defer done()
		ctx = ctxTimeout
	}
	op(ctx)
}
