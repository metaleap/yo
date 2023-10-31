package crontab

import (
	"encoding/json"
	"testing"
)

func TestCrontab(t *testing.T) {
	for src, cmp := range map[string]Expr{
		"5,3-27/4,7/2 4/3 7/2 feb-sep 4,sun-tu": &expr{
			Minutes:     Field{{0, 5, 5}, {4, 3, 27}, {2, 7, 59}},
			Hours:       Field{{3, 4, 59}},
			DaysOfMonth: Field{{2, 7, 31}},
			Months:      Field{{0, 2, 9}},
			DaysOfWeek:  Field{{0, 4, 4}, {0, 0, 2}},
		},
	} {
		crontab, err := Parse(src)
		if err != nil {
			t.Error(err)
		} else if s1, s2 := toJSON(crontab), toJSON(cmp); s1 != s2 {
			t.Errorf("%s\n%s\n", src, s1)
		}
	}
}

func toJSON(it any) string {
	data, _ := json.MarshalIndent(it, "  ", "  ")
	return string(data)
}
