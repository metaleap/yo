package util

import "yo/util/str"

type Err string

var errSubstrToHttpStatusCode = map[string]int{
	"AlreadyExists": 409,
	"NotFound":      404,
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
