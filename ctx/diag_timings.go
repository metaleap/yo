package yoctx

import (
	"time"

	. "yo/util"
)

type Timing struct {
	Step string
	Time int64 // careful! actually a timestamp in `Now`, all converted to Duration only in `Done`
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
	ret.Step(firstStep)
	return &ret
}

func (me *timings) Step(step string) {
	if !me.noOp {
		me.steps = append(me.steps, Timing{Step: step, Time: time.Now().UnixNano()})
	}
}

func (me *timings) AllDone() (total int64, steps []Timing) {
	if me.noOp {
		return 0, nil
	}
	now := time.Now().UnixNano()
	total = (now - me.steps[0].Time) // this is still actually still a timestemp, until:
	for i := range me.steps {        // converting timestamps into the actual durations only now
		if i == len(me.steps)-1 {
			me.steps[i].Time = (now - me.steps[i].Time)
		} else {
			me.steps[i].Time = (me.steps[i+1].Time - me.steps[i].Time)
		}
	}
	steps, me.steps = me.steps, nil
	return
}