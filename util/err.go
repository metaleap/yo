package util

type Err string

func (me Err) Error() string { return string(me) }
