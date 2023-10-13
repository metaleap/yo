package util

import "yo/util/str"

type Err string

var errSubstrToHttpStatusCode = map[string]int{
	"Required":      400,
	"Expected":      400,
	"NotFound":      404,
	"AlreadyExists": 409,
	"Timeout":       504,
	"TimedOut":      504,
}

func (me Err) Error() string { return string(me) }

func (me Err) HttpStatusCode() int {
	for substr, code := range errSubstrToHttpStatusCode {
		if str.Has(string(me), substr) {
			return code
		}
	}
	return 400
}
