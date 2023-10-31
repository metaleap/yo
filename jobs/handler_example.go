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
		func(string, string) exampleHandler { return exampleHandler{} })
}

type exampleJobDetails struct {
	MsgFmt string `json:"msg_fmt" bson:"msg_fmt"`
}

type exampleJobResults struct {
	NumLoggingsDone int `json:"num_done" bson:"num_done"`
}

type exampleTaskDetails struct {
	Time time.Time `json:"time" bson:"time"`
}

type exampleTaskResults struct {
	NumLoggingsDone int `json:"num_done" bson:"num_done"`
}

func (exampleHandler) dice() byte {
	var b [1]byte
	_, _ = rand.Reader.Read(b[:])
	return b[0]
}

func (it exampleHandler) JobDetails(ctx *Context) (JobDetails, error) {
	if it.dice()%2 == 0 {
		return &exampleJobDetails{MsgFmt: ">>>>>>>>>>>>>IT WAS %s JUST %s AGO"}, nil
	}
	return ctx.JobDetails, nil
}

func (exampleHandler) TaskDetails(_ *Context, stream chan<- []TaskDetails, _ func(error) error) (*ListRequest, *TaskFilter) {
	stream <- []TaskDetails{&exampleTaskDetails{Time: time.Now()}}
	stream <- []TaskDetails{
		&exampleTaskDetails{Time: time.Now().Add(-365 * 24 * time.Hour)},
		&exampleTaskDetails{Time: time.Now().Add(-30 * 24 * time.Hour)},
	}
	return nil, nil
}

func (it exampleHandler) TaskResults(ctx *Context, task TaskDetails) (TaskResults, error) {
	log := loggerFor(ctx)
	msg := ctx.JobDetails.(*exampleJobDetails).MsgFmt
	t := task.(*exampleTaskDetails).Time
	if d := it.dice(); d%11 == 0 {
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
