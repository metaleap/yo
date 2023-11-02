package yojobs

import (
	"errors"
	"time"

	. "yo/ctx"
)

type JobTask struct {
	Id string

	JobTypeId  string
	JobRunId   string
	State      RunState
	StartTime  *time.Time
	FinishTime *time.Time
	Attempts   []*TaskAttempt
	Version    int

	Details TaskDetails
	Results TaskResults

	jobRun *JobRun
}

func (me *JobTask) Failed() bool {
	return (me.State == Done) && (len(me.Attempts) > 0) && (me.Attempts[0].Err != nil)
}

func (me *JobTask) Succeeded() bool {
	return (me.State == Done) && (len(me.Attempts) > 0) && (me.Attempts[0].Err == nil)
}

func (me *JobTask) markForRetryOrAsFailed(jobDef *JobDef) (retry bool) {
	if (jobDef != nil) && (len(me.Attempts) <= int(jobDef.MaxTaskRetries)) { // first attempt was not a RE-try
		me.State, me.StartTime, me.FinishTime = Pending, nil, nil
		return true
	}
	me.State, me.FinishTime = Done, timeNow()
	return false
}

type TaskAttempt struct {
	Time      time.Time
	TaskError *TaskError

	// Err is the `error` equivalent of `TaskError`. For read accesses, both can be used interchangably. Write accesses (that last) don't occur outside this package.
	Err error `json:"-"` // code in this package uses only `Err`, not `TaskError` which is just for storage and only used in un/marshaling hooks and API mapping code.
}

type TaskError struct {
	Message string
}

func (me *TaskError) Err() error {
	if me == nil {
		return nil
	}
	return errors.New(me.Message)
}

func (me *TaskError) Error() (s string) {
	if me != nil {
		s = me.Err().Error()
	}
	return
}

// Timeout implements utils.HasTimeout
func (me *JobTask) Timeout(ctx *Ctx) time.Duration {
	var job_def *JobDef
	if me.jobRun != nil {
		job_def = me.jobRun.JobDef.Get(ctx)
	}
	if (job_def != nil) && (job_def.TimeoutTaskRunSecs) > 0 {
		return time.Second * time.Duration(job_def.TimeoutTaskRunSecs)
	}
	return TimeoutLong
}
