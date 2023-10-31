package jobs

import (
	"cmp"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"time"

	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

type hasId interface{ GetId() string }

func newId(prefix string) string {
	return prefix + "_" + str.FromI64(time.Now().In(Timezone).UnixNano(), 36) + "_" + str.FromI64(rand.Int63n(math.MaxInt64), 36)
}

func findByID[T hasId](collection []T, id string) T {
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
