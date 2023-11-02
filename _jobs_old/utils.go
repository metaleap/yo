package _yo_jobs_old

import (
	"cmp"
	"context"
	"errors"
	"math"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

func newId(prefix string) string {
	ret, _ := os.Hostname()
	for _, n := range []int64{time.Now().In(Timezone).UnixNano(), rand.Int63n(math.MaxInt64), int64(os.Getpid()), int64(os.Getppid())} {
		ret += ("_" + str.FromI64(n, 36))
	}
	return prefix + "_" + ret
}

func ensureTz(times ...*time.Time) {
	for _, t := range times {
		if t != nil {
			*t = t.In(Timezone)
		}
	}
}

func errNotFoundJobRun(id string) error {
	return errors.New(str.Fmt("job run '%s' no longer exists", id))
}

func errNotFoundJobDef(id string) error {
	return errors.New(str.Fmt("job def '%s' renamed or removed in configuration", id))
}

func errNotFoundJobType(jobDefId string, jobTypeId string) error {
	return errors.New(str.Fmt("job def '%s' type '%s' renamed or removed", jobDefId, jobTypeId))
}

func firstNonNil[T any](collection ...*T) (found *T) {
	return sl.FirstWhere(collection, func(it *T) bool { return (it != nil) })
}

func timeNow() *time.Time { return ToPtr(time.Now().In(Timezone)) }

func sanitize[TStruct any, TField cmp.Ordered](min TField, max TField, parse func(string) (TField, error), fields map[string]*TField) (err error) {
	type_of_struct := ReflType[TStruct]()
	for field_name, field_ptr := range fields {
		if clamped := Clamp(min, max, *field_ptr); clamped != *field_ptr {
			field, _ := type_of_struct.FieldByName(field_name)
			if *field_ptr, err = parse(field.Tag.Get("default")); err != nil {
				return
			}
		}
	}
	return
}

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
	callOp := func(ctx context.Context, item T) {
		timeout := opTimeout
		if customTimeout, _ := (any(item)).(HasTimeout); timeout == 0 && customTimeout != nil {
			timeout = customTimeout.Timeout()
		}
		op(ctx, item)
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
