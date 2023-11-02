package yojobs

import (
	"cmp"
	"errors"

	yodb "yo/db"
	. "yo/util"
	"yo/util/str"
)

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
