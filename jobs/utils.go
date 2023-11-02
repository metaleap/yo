package yojobs

import (
	"cmp"
	"errors"
	"time"

	yodb "yo/db"
	. "yo/util"
	"yo/util/str"
)

func timeNow() *time.Time { return ToPtr(time.Now().In(Timezone)) }

func errNotFoundJobRun(id string) error {
	return errors.New(str.Fmt("job run '%s' no longer exists", id))
}

func errNotFoundJobDef(id string) error {
	return errors.New(str.Fmt("job def '%s' renamed or removed in configuration", id))
}

func errNotFoundJobType(jobDefId yodb.Text, jobTypeId yodb.Text) error {
	return errors.New(str.Fmt("job def '%s' type '%s' renamed or removed", jobDefId, jobTypeId))
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
