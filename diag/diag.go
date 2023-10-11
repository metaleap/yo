package yodiag

import (
	"time"

	. "yo/util"
)

type Timing struct {
	Step     string
	Duration int64 // careful! actually a timestamp in `Now`, all converted to Duration only in `Done`
}

type timings struct {
	steps []Timing
	noOp  bool
}

type Timings interface {
	Step(step string)
	AllDone() (total int64, steps []Timing)
}

func NewTimings(firstStep string, noOp bool) Timings {
	ret := timings{steps: make([]Timing, 0, If(noOp, 0, 8))}
	if !noOp {
		ret.steps = append(ret.steps, Timing{Step: firstStep, Duration: time.Now().UnixNano()})
	}
	return &ret
}

func (me *timings) Step(step string) {
	if !me.noOp {
		me.steps = append(me.steps, Timing{Step: step, Duration: time.Now().UnixNano()})
	}
}

func (me *timings) AllDone() (total int64, steps []Timing) {
	if me.noOp {
		return 0, nil
	}
	now := time.Now().UnixNano()
	total = (now - me.steps[0].Duration) // this "Duration" is still actually a timestemp, until:
	for i := range me.steps {            // converting timestamps temporarily stored in `Duration` into the actual durations only now
		if i == len(me.steps)-1 {
			me.steps[i].Duration = (now - me.steps[i].Duration)
		} else {
			me.steps[i].Duration = (me.steps[i+1].Duration - me.steps[i].Duration)
		}
	}
	steps, me.steps = me.steps, nil
	return
}
