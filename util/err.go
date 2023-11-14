package util

import "yo/util/str"

type Err string

var errSubstrToHttpStatusCode = map[string]int{
	"DoesNotExist":  400, // no 404 wanted for those, there's NotFound below for that
	"ContentLength": 400,
	"WrongPassword": 401,
	"MustBeAdmin":   401,
	"Unauthorized":  403,
	"Forbidden":     403,
	"NotFound":      404,
	"AlreadyExists": 409,
	"Required":      422,
	"Expected":      422,
	"Invalid":       422,
	"TooShort":      422,
	"TooLong":       422,
	"TooLow":        422,
	"TooHigh":       422,
	"TooSmall":      422,
	"TooBig":        422,
	"NotStored":     502,
	"Timeout":       504,
	"TimedOut":      504,
}

func (me Err) Error() string  { return string(me) }
func (me Err) String() string { return string(me) }
func (me Err) AsAny() any     { return me }
func (me Err) HttpStatusCodeOr(preferredDefault int) int {
	for substr, code := range errSubstrToHttpStatusCode {
		if str.Has(string(me), substr) {
			return code
		}
	}
	return preferredDefault
}

func Try(do func(), catch func(any)) {
	defer func() {
		if fail := recover(); (fail != nil) && (catch != nil) {
			catch(fail)
		}
	}()
	do()
}
