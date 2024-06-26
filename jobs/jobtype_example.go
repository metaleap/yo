package yojobs

import (
	"crypto/rand"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	. "yo/util"
	"yo/util/str"
)

const ExampleJobEnabled = false

var ExampleJobDef = JobDef{
	Name:                             "exampleJob",
	JobTypeId:                        "yojobs.ExampleJobType",
	MaxTaskRetries:                   2,
	DeleteAfterDays:                  1,
	Disabled:                         yodb.Bool(!ExampleJobEnabled),
	TimeoutSecsTaskRun:               2,
	TimeoutSecsJobRunPrepAndFinalize: 4,
	Schedules:                        ScheduleOncePerMinute,
}

type ExampleJobType struct{}

func init() {
	if ExampleJobEnabled {
		Register[ExampleJobType, exampleJobDetails, exampleJobResults, exampleTaskDetails, exampleTaskResults](
			func(string) ExampleJobType { return ExampleJobType{} })
	}
}

type exampleJobDetails struct {
	MsgFmt string
}

type exampleJobResults struct {
	NumLoggingsDone int
}

type exampleTaskDetails struct {
	Time time.Time
}

type exampleTaskResults struct {
	NumLoggingsDone int
}

func (ExampleJobType) dice() byte {
	var b [1]byte
	_, _ = rand.Reader.Read(b[:])
	return b[0]
}

func (me ExampleJobType) JobDetails(ctx *Ctx) JobDetails {
	return &exampleJobDetails{MsgFmt: If(((me.dice() % 2) == 0), "<<<<<IT WAS %s JUST %s AGO", ">>>>>IT WAS %s JUST %s AGO")}
}

func (ExampleJobType) TaskDetails(_ *Ctx, stream func([]TaskDetails)) {
	stream([]TaskDetails{&exampleTaskDetails{Time: time.Now()}})
	stream([]TaskDetails{
		&exampleTaskDetails{Time: time.Now().Add(-365 * 24 * time.Hour)},
		&exampleTaskDetails{Time: time.Now().Add(-30 * 24 * time.Hour)},
	})
}

func (me ExampleJobType) TaskResults(ctx *Ctx, task TaskDetails) TaskResults {
	msg := ctx.Job.Details.(*exampleJobDetails).MsgFmt
	t := task.(*exampleTaskDetails).Time
	if d := me.dice(); (d % 11) == 0 {
		panic(str.Fmt("artificially provoked random error due to dice throw %d", d))
	}
	println(str.Fmt(msg, t.Format("2006-01-02 15:04:05"), time.Since(t)))
	return &exampleTaskResults{NumLoggingsDone: 1}
}

func (ExampleJobType) JobResults(_ *Ctx) (stream func(func() *Ctx, *JobTask, *bool), results func() JobResults) {
	var num int
	stream = func(_ func() *Ctx, task *JobTask, abort *bool) {
		num += task.Results.(*exampleTaskResults).NumLoggingsDone
	}
	results = func() JobResults {
		return &exampleJobResults{NumLoggingsDone: num}
	}
	return
}
