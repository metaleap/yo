package util

import (
	"time"
)

func DtAtZeroSecsUtc(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}
