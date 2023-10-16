package util

import "yo/util/str"

type Err string

var errSubstrToHttpStatusCode = map[string]int{
	"Required":     400,
	"Expected":     400,
	"Invalid":      400,
	"TooShort":     400,
	"TooLong":      400,
	"TooLow":       400,
	"TooHigh":      400,
	"TooSmall":     400,
	"TooBig":       400,
	"DoesNotExist": 400,

	"AlreadyExists": 409,
	"NotFound":      404,
	"Unauthorized":  403,
	"Forbidden":     403,
	"WrongPassword": 401,
	"MustBeAdmin":   401,
	"NotStored":     502,

	"Timeout":  504,
	"TimedOut": 504,
}

func (me Err) Error() string  { return string(me) }
func (me Err) String() string { return string(me) }
func (me Err) HttpStatusCodeOr(preferredDefault int) int {
	for substr, code := range errSubstrToHttpStatusCode {
		if str.Has(string(me), substr) {
			return code
		}
	}
	return preferredDefault
}
