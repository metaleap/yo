package crontab

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"yo/util/sl"
	"yo/util/str"
)

type fieldParser func(string) (Field, error)

var (
	parseMins        = fieldParserFor("minutes", 0, 59, false, nil)
	parseHours       = fieldParserFor("hours", 0, 59, false, nil)
	parseDaysOfMonth = fieldParserFor("month days", 1, 31, false, nil)
	parseMonths      = fieldParserFor("months", 1, 12, false, valueNames(1, 12, func(month int) []string {
		s := strings.ToLower(time.Month(month).String())
		return []string{s, s[:3]}
	}))
	parseDaysOfWeek = fieldParserFor("week days", 0, 6, true, valueNames(0, 6, func(weekDay int) []string {
		s := strings.ToLower(time.Weekday(weekDay).String())
		return []string{s, s[:3], s[:2]}
	}))
)

func Parse(src string) (_ Expr, err error) {
	switch src { // wikiless.org/wiki/Crontab#Nonstandard_predefined_scheduling_definitions
	case "@yearly", "@annually":
		src = "0 0 1 jan *"
	case "@monthly":
		src = "0 0 1 * *"
	case "@weekly":
		src = "0 0 * * sun"
	case "@daily", "@midnight", "@nightly":
		src = "0 0 * * *"
	case "@hourly":
		src = "0 * * * *"
	}

	fields := strings.Split(strings.TrimSpace(src), " ")
	if fields = sl.Without(fields, ""); len(fields) > 5 {
		return nil, errors.New(str.Fmt("expected at most 5 fields in crontab expression '%s', not %d", src, len(fields)))
	}
	fields = append(fields, sl.Repeat(5-len(fields), "*")...)

	ret := &expr{}
	if ret.Minutes, err = parseMins(fields[0]); err == nil {
		if ret.Hours, err = parseHours(fields[1]); err == nil {
			if ret.DaysOfMonth, err = parseDaysOfMonth(fields[2]); err == nil {
				if ret.Months, err = parseMonths(fields[3]); err == nil {
					ret.DaysOfWeek, err = parseDaysOfWeek(fields[4])
				}
			}
		}
	}
	return ret, err
}

func fieldParserFor(fieldName string, valueMin int, valueMax int, modBeyond bool, valueNames map[string]int) fieldParser {
	return func(src string) (ret Field, err error) {
		if src == "*" {
			return Field{{EveryNth: 1, From: valueMin, Through: valueMax}}, nil
		}
		for _, src := range strings.Split(src, ",") {
			var this FieldItem
			if idxSlash := strings.IndexByte(src, '/'); idxSlash > 0 {
				this.EveryNth, err = parseValue(src[idxSlash+1:], fieldName+"/n", 1, uint64(math.MaxInt32), false, nil)
				if src = src[:idxSlash]; err != nil {
					return
				}
			}
			if idxDash := strings.IndexByte(src, '-'); idxDash <= 0 {
				if src == "*" {
					this.From = valueMin
				} else {
					this.From, err = parseValue(src, fieldName, uint64(valueMin), uint64(valueMax), modBeyond, valueNames)
				}
				if this.Through = this.From; this.EveryNth > 0 {
					this.Through = valueMax
				}
			} else {
				this.From, err = parseValue(src[:idxDash], fieldName, uint64(valueMin), uint64(valueMax), modBeyond, valueNames)
				if err == nil {
					this.Through, err = parseValue(src[idxDash+1:], fieldName, uint64(valueMin), uint64(valueMax), modBeyond, valueNames)
				}
				if err == nil && this.From > this.Through {
					err = errors.New(str.Fmt("range %d-%d start %d must be before end %d", this.From, this.Through, this.From, this.Through))
				}
			}
			if err != nil {
				return
			}
			ret = append(ret, this)
		}
		return
	}
}

func parseValue(src string, fieldName string, valueMin uint64, valueMax uint64, modBeyond bool, valueNames map[string]int) (int, error) {
	var n int
	var found bool
	if valueNames != nil {
		n, found = valueNames[strings.ToLower(src)]
	}
	if !found {
		n64, err := strconv.ParseUint(src, 10, 32)
		if err != nil {
			return 0, errors.New(str.Fmt("field '%s' value '%s' faulty: %s", fieldName, src, err))
		}
		if n64 < valueMin || n64 > valueMax {
			if modBeyond {
				n64 = valueMin + ((n64 - valueMin) % (1 + (valueMax - valueMin)))
			} else {
				return 0, errors.New(str.Fmt("expected '%s' field to be in the range %d-%d but got %d", fieldName, valueMin, valueMax, n))
			}
		}
		n = int(n64)
	}
	return n, nil
}

func valueNames(valueMin int, valueMax int, altNames func(int) []string) (ret map[string]int) {
	ret = map[string]int{}
	for i := valueMin; i <= valueMax; i++ {
		for _, s := range altNames(i) {
			ret[s] = i
		}
	}
	return
}
