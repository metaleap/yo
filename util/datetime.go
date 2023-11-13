package util

import (
	"time"
)

var DoAfter = time.AfterFunc

func DtAtZeroSecsUtc(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}

func DtAtZeroNanosUtc(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

func Duration(timeNanos ...int64) time.Duration {
	min, max := Min(timeNanos...), Max(timeNanos...)
	return time.Duration(max - min)
}
