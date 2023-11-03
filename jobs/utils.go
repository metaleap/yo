package yojobs

import (
	"cmp"
	"errors"
	"sync/atomic"
	"time"

	yodb "yo/db"
	q "yo/db/query"
	. "yo/util"
	"yo/util/str"
)

func qById(id yodb.I64) q.Query {
	return yodb.ColID.Equal(id)
}

func errNotFoundJobRun(id yodb.I64) error {
	return errors.New(str.Fmt("job run '%d' no longer exists", id))
}

func errNotFoundJobDef(jobRunId yodb.I64) error {
	return errors.New(str.Fmt("job def of job run %d renamed or removed in configuration", jobRunId))
}

func errNotFoundJobType(jobDefName yodb.Text, jobTypeId yodb.Text) error {
	return errors.New(str.Fmt("job def '%s' type '%s' renamed or removed", jobDefName, jobTypeId))
}

func sanitizeOptionsFields[TStruct any, TField cmp.Ordered](min TField, max TField, parse func(string) (TField, error), fields map[string]*TField) (err error) {
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

// GoItems is like a `sync.WaitGroup` when `maxConcurrentOps` is 0, but a semaphore with a `maxConcurrentOps` greater than 1.
func GoItems[T any](workSet []T, do func(T), maxConcurrentOps int) {
	if len(workSet) == 0 {
		return
	}

	if maxConcurrentOps == 1 || len(workSet) == 1 {
		for _, item := range workSet {
			do(item)
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
			do(item)
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
