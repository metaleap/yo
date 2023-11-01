package jobs

import (
	"crypto/rand"
	"errors"
	"time"

	"yo/util/str"
)

type exampleHandler struct{}

func init() {
	_ = Register[exampleHandler, exampleJobDetails, exampleJobResults, exampleTaskDetails, exampleTaskResults](
		func(string) exampleHandler { return exampleHandler{} })
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

func (exampleHandler) dice() byte {
	var b [1]byte
	_, _ = rand.Reader.Read(b[:])
	return b[0]
}

func (it exampleHandler) IsTaskErrRetryable(error) bool { return false }

func (it exampleHandler) JobDetails(ctx *Context) (JobDetails, error) {
	if (it.dice() % 2) == 0 {
		return &exampleJobDetails{MsgFmt: ">>>>>>>>>>>>>IT WAS %s JUST %s AGO"}, nil
	}
	return ctx.JobDetails, nil
}

func (exampleHandler) TaskDetails(_ *Context, stream chan<- []TaskDetails, _ func(error) error) (*ListRequest, *JobTaskFilter) {
	stream <- []TaskDetails{&exampleTaskDetails{Time: time.Now()}}
	stream <- []TaskDetails{
		&exampleTaskDetails{Time: time.Now().Add(-365 * 24 * time.Hour)},
		&exampleTaskDetails{Time: time.Now().Add(-30 * 24 * time.Hour)},
	}
	return nil, nil
}

func (it exampleHandler) TaskResults(ctx *Context, task TaskDetails) (TaskResults, error) {
	log := loggerNew()
	msg := ctx.JobDetails.(*exampleJobDetails).MsgFmt
	t := task.(*exampleTaskDetails).Time
	if d := it.dice(); (d % 11) == 0 {
		return nil, errors.New(str.Fmt("artificially provoked random error due to dice throw %d", d))
	}
	log.Infof(msg, t.Format("2006-01-02 15:04:05"), time.Since(t))
	return &exampleTaskResults{NumLoggingsDone: 1}, nil
}

func (exampleHandler) JobResults(_ *Context, tasks func() <-chan *Task) (JobResults, error) {
	var num int
	for task := range tasks() {
		if results, _ := task.Results.(*exampleTaskResults); results != nil {
			num += results.NumLoggingsDone
		}
	}
	return &exampleJobResults{NumLoggingsDone: num}, nil
}
