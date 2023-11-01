package yojobs

import (
	"errors"
	"time"
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

	Details TaskDetails `json:"-"`
	Results TaskResults `json:"-"`
	// DetailsStore is for storage and not to be used in code outside internal un/marshaling hooks, use `Details`.
	DetailsStore map[string]any
	// ResultsStore is for storage and not to be used in code outside internal un/marshaling hooks, use `Results`.
	ResultsStore map[string]any

	jobRun *JobRun
}

func (it *JobTask) Failed() bool {
	return (it.State == Done) && (len(it.Attempts) > 0) && (it.Attempts[0].Err != nil)
}

func (it *JobTask) Succeeded() bool {
	return (it.State == Done) && (len(it.Attempts) > 0) && (it.Attempts[0].Err == nil)
}

func (it *JobTask) JobDef() string {
	if it.jobRun == nil {
		return ""
	}
	return it.jobRun.JobDefId
}

func (it *JobTask) markForRetryOrAsFailed(jobDef *JobDef) (retry bool) {
	if (jobDef != nil) && (len(it.Attempts) <= jobDef.TaskRetries) { // first attempt was not a RE-try
		it.State, it.StartTime, it.FinishTime = Pending, nil, nil
		return true
	}
	it.State, it.FinishTime = Done, timeNow()
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

func (it *TaskError) Err() error {
	if it == nil {
		return nil
	}
	return errors.New(it.Message)
}

func (it *TaskError) Error() (s string) {
	if it != nil {
		s = it.Err().Error()
	}
	return
}

// Timeout implements utils.HasTimeout
func (it *JobTask) Timeout() time.Duration {
	if (it.jobRun != nil) && (it.jobRun.jobDef) != nil && (it.jobRun.jobDef.Timeouts.TaskRun) > 0 {
		return it.jobRun.jobDef.Timeouts.TaskRun
	}
	return TimeoutLong
}
