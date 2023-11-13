package crontab

import (
	"fmt"
	"time"

	"yo/util/sl"
	"yo/util/str"
)

// Expr is obtained via `Parse(string) (Expr, error)`.
type Expr interface {
	fmt.Stringer

	DateOk(time.Time) bool
	TimeOk(time.Time) bool
	DateAndTimeOk(time.Time) bool

	// SoonestTo searches from `now` for the closest satisfactory minute in the past and future.
	// The optional `after` and `before` parameters, if both non-`nil`, place bounds on the search. Beware that both pointers are written to, to be `In()` the same `Time.Location` as `now` is.
	// Either return value may be `nil`, but both being `nil` is extremely unlikely with `Expr`s resulting from successful `Parse`s (of not-too-outlandish inputs), and with sufficiently distant `before`/`after`.
	// Non-`nil` return values are guaranteed to be between `after` and `before` and not equal to them.
	SoonestTo(now time.Time, after *time.Time, before *time.Time) (beforeNow *time.Time, afterNow *time.Time)
}

type expr struct {
	Minutes     Field
	Hours       Field
	DaysOfMonth Field
	Months      Field
	DaysOfWeek  Field
}

type Field []FieldItem

// field names like in the human-readable descriptions used at https:/crontab.guru
type FieldItem struct {
	EveryNth int
	From     int
	Through  int
}

func (it *FieldItem) String() (s string) {
	if it.From == it.Through {
		s = str.FromInt(it.From)
	} else {
		s = str.FromInt(it.From) + "-" + str.FromInt(it.Through)
	}
	if it.EveryNth > 0 {
		s += "/" + str.FromInt(it.EveryNth)
	}
	return
}

func (it Field) String() string {
	return str.Join(sl.As(it, func(f FieldItem) string { return f.String() }), ",")
}

func (it *expr) String() string {
	return str.Join(sl.As([]Field{
		it.Minutes,
		it.Hours,
		it.DaysOfMonth,
		it.Months,
		it.DaysOfWeek,
	}, Field.String), " ")
}

func newDateFrom(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	y, m, d := t.Date()
	ret := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	return &ret
}

func (it *expr) SoonestTo(now time.Time, after *time.Time, before *time.Time) (beforeNow *time.Time, afterNow *time.Time) {
	tz := now.Location()
	normalize := func(t time.Time) time.Time { // set t to its hh:mm:00.000000000
		t = t.In(tz)
		y, m, d := t.Date()
		hour, min, _ := t.Clock()
		return time.Date(y, m, d, hour, min, 0, 0, tz)
	}
	now = normalize(now)
	if after != nil {
		*after = normalize(*after)
	}
	if before != nil {
		*before = normalize(*before)
	}

	for dir, ret := range map[int]**time.Time{
		-1: &beforeNow,
		1:  &afterNow,
	} {
		t := now
		move_on := func() bool {
			return (!it.DateOk(t)) && ((after == nil) || t.After(*after)) && ((before == nil) || t.Before(*before))
		}
		for move_on() { // jump to right search-starting day first:
			t = t.AddDate(0, 0, dir)
		}

		for date := newDateFrom(&t); true; t = t.Add(time.Minute * time.Duration(dir)) {
			// has `t` just jumped to another date? if so & it's not `DateOk`, let's jump right to the next OK day.
			if date.Day() != t.Day() {
				date = newDateFrom(&t)
				if t = *date; dir < 0 { // t hereby at 00:00, but if going past, we'd want to be at 23:59
					t = t.Add(time.Hour*23 + time.Minute*59)
				}
				for move_on() {
					adj_day := date.AddDate(0, 0, dir)
					date, t = &adj_day, adj_day
				}
			}
			// now actually check `t`
			if (after != nil) && (t.Before(*after) || t.Equal(*after)) {
				if t = *after; dir < 0 {
					break // going pastwards, the problem will remain, so break out
				}
				continue // matters here, we dont want to return `after`
			}
			if (before != nil) && (t.After(*before) || t.Equal(*before)) {
				if t = *before; dir > 0 {
					break // going forward, the problem will remain, so break out
				}
				continue // matters here, we dont want to return `before`
			}
			if it.DateAndTimeOk(t) {
				*ret = &t
				break
			}
		}
	}
	return beforeNow, afterNow
}

func (it *expr) DateOk(t time.Time) bool {
	ok, _ := it.ok(t, true, false)
	return ok
}

func (it *expr) TimeOk(t time.Time) bool {
	_, ok := it.ok(t, false, true)
	return ok
}

func (it *expr) DateAndTimeOk(t time.Time) bool {
	ok_date, ok_time := it.ok(t, true, true)
	return ok_date && ok_time
}

func (it *expr) ok(t time.Time, checkDay bool, checkTime bool) (dayOk bool, timeOk bool) {
	dayOk, timeOk = true, true
	_, month, day := t.Date()
	hour, min, _ := t.Clock()
	for _, check := range []struct {
		field Field
		value int
		ret   *bool
	}{
		{it.Hours, hour, &timeOk},
		{it.Minutes, min, &timeOk},
		{it.Months, int(month), &dayOk},
		{it.DaysOfMonth, day, &dayOk},
		{it.DaysOfWeek, int(t.Weekday()), &dayOk},
	} {
		if ((check.ret == &dayOk) && !checkDay) || ((check.ret == &timeOk) && !checkTime) {
			continue
		}
		if !sl.Any(check.field, func(it FieldItem) bool { return it.ok(check.value) }) {
			*check.ret = false
			return
		}
	}
	return
}

func (it *FieldItem) ok(n int) bool {
	return ((it.EveryNth == 0) || ((n % it.EveryNth) == 0)) && (n >= it.From) && (n <= it.Through)
}
