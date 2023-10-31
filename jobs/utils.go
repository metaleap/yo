package jobs

import (
	"cmp"
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"time"

	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

type hasID interface{ GetId() string }
type HasTimeout interface{ Timeout() time.Duration }

func atOnceDo(f ...func()) {
	concurrentlyDo(ctxNone, f, func(ctx context.Context, f func()) { f() }, 0, 0)
}

// concurrentlyDo is like a `sync.WaitGroup` when `maxConcurrentOps`
// is 0, but a semaphore with a `maxConcurrentOps` greater than 1.
// If `T` implements `hasTimeout` and `opTimeout` is 0, only `T.Timeout()` is used.
// If `opTimeout` is greater than `0`, this indicates that `T.Timeout()` is to be ignored for this particular run of `op`s.
func concurrentlyDo[T any](ctx context.Context, workSet []T, op func(context.Context, T), maxConcurrentOps int, opTimeout time.Duration) {
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
		WithTimeoutDo(ctx, timeout, func(ctx context.Context) {
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

func findByID[T hasID](collection []T, id string) T {
	return sl.FirstWhere(collection, func(v T) bool { return v.GetId() == id })
}

func errNotFoundJob(id string) error {
	return errors.New(str.Fmt("job '%s' no longer exists", id))
}

func errNotFoundSpec(id string) error {
	return errors.New(str.Fmt("job spec '%s' renamed or removed in configuration", id))
}

func errNotFoundHandler(specID string, handlerID string) error {
	return errors.New(str.Fmt("job spec '%s' handler '%s' renamed or removed", specID, handlerID))
}

func firstNonNil[T any](collection ...*T) (found *T) {
	return sl.FirstWhere(collection, func(it *T) bool { return it != nil })
}

func timeNow() *time.Time { return ToPtr(time.Now().In(Timezone)) }

func clamp[T cmp.Ordered](min T, max T, i T) T {
	return If(i > max, max, If(i < min, min, i))
}

func sanitize[TStruct any, TField cmp.Ordered](min TField, max TField, parse func(string) (TField, error), fields map[string]*TField) (err error) {
	typeOfStruct := typeOf[TStruct]()
	for fieldName, fieldPtr := range fields {
		if clamped := clamp(min, max, *fieldPtr); clamped != *fieldPtr {
			field, _ := typeOfStruct.FieldByName(fieldName)
			if *fieldPtr, err = parse(field.Tag.Get("default")); err != nil {
				return
			}
		}
	}
	return
}

func typeOf[T any]() reflect.Type {
	var none T
	return reflect.TypeOf(none)
}
