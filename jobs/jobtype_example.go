package yojobs

import (
	"crypto/rand"
	"errors"
	"time"

	"yo/util/str"
)

type ExampleJobType struct{}

func init() {
	Register[ExampleJobType, exampleJobDetails, exampleJobResults, exampleTaskDetails, exampleTaskResults](
		func(string) ExampleJobType { return ExampleJobType{} })
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

func (ExampleJobType) IsTaskErrRetryable(error) bool { return false }

func (me ExampleJobType) JobDetails(ctx *Context) (JobDetails, error) {
	if (me.dice() % 2) == 0 {
		return &exampleJobDetails{MsgFmt: ">>>>>>>>>>>>>IT WAS %s JUST %s AGO"}, nil
	}
	return ctx.JobDetails, nil
}

func (ExampleJobType) TaskDetails(_ *Context, stream chan<- []TaskDetails, _ func(error) error) {
	stream <- []TaskDetails{&exampleTaskDetails{Time: time.Now()}}
	stream <- []TaskDetails{
		&exampleTaskDetails{Time: time.Now().Add(-365 * 24 * time.Hour)},
		&exampleTaskDetails{Time: time.Now().Add(-30 * 24 * time.Hour)},
	}
}

func (me ExampleJobType) TaskResults(ctx *Context, task TaskDetails) (TaskResults, error) {
	msg := ctx.JobDetails.(*exampleJobDetails).MsgFmt
	t := task.(*exampleTaskDetails).Time
	if d := me.dice(); (d % 11) == 0 {
		return nil, errors.New(str.Fmt("artificially provoked random error due to dice throw %d", d))
	}
	println(str.Fmt(msg, t.Format("2006-01-02 15:04:05"), time.Since(t)))
	return &exampleTaskResults{NumLoggingsDone: 1}, nil
}

func (ExampleJobType) JobResults(_ *Context, tasks func() <-chan *JobTask) (JobResults, error) {
	var num int
	for task := range tasks() {
		if results, _ := task.Results.(*exampleTaskResults); results != nil {
			num += results.NumLoggingsDone
		}
	}
	return &exampleJobResults{NumLoggingsDone: num}, nil
}
