package jobs

import (
	"cmp"
	"errors"
	"math"
	"math/rand"
	"os"
	"time"

	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

type hasId interface{ GetId() string }

func newId(prefix string) string {
	ret, _ := os.Hostname()
	for _, n := range []int64{time.Now().In(Timezone).UnixNano(), rand.Int63n(math.MaxInt64), int64(os.Getpid()), int64(os.Getppid())} {
		ret += ("_" + str.FromI64(n, 36))
	}
	return prefix + "_" + ret
}

func ensureTZ(times ...*time.Time) {
	for _, t := range times {
		if t != nil {
			*t = t.In(Timezone)
		}
	}
}

func findById[T hasId](collection []T, id string) T {
	return sl.FirstWhere(collection, func(v T) bool { return v.GetId() == id })
}

func errNotFoundJob(id string) error {
	return errors.New(str.Fmt("job '%s' no longer exists", id))
}

func errNotFoundDef(id string) error {
	return errors.New(str.Fmt("job def '%s' renamed or removed in configuration", id))
}

func errNotFoundHandler(defID string, handlerID string) error {
	return errors.New(str.Fmt("job def '%s' handler '%s' renamed or removed", defID, handlerID))
}

func firstNonNil[T any](collection ...*T) (found *T) {
	return sl.FirstWhere(collection, func(it *T) bool { return it != nil })
}

func timeNow() *time.Time { return ToPtr(time.Now().In(Timezone)) }

func sanitize[TStruct any, TField cmp.Ordered](min TField, max TField, parse func(string) (TField, error), fields map[string]*TField) (err error) {
	typeOfStruct := ReflType[TStruct]()
	for fieldName, fieldPtr := range fields {
		if clamped := Clamp(min, max, *fieldPtr); clamped != *fieldPtr {
			field, _ := typeOfStruct.FieldByName(fieldName)
			if *fieldPtr, err = parse(field.Tag.Get("default")); err != nil {
				return
			}
		}
	}
	return
}
