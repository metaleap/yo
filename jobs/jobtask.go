package yojobs

import (
	"time"

	. "yo/ctx"
	yodb "yo/db"
)

type JobTask struct {
	Id      yodb.I64
	Version yodb.U32

	JobTypeId  yodb.Text
	JobRun     yodb.Ref[JobRun, yodb.RefOnDelCascade]
	state      yodb.Text
	StartTime  *yodb.DateTime
	FinishTime *yodb.DateTime
	Attempts   yodb.JsonArr[TaskAttempt]

	Details TaskDetails
	Results TaskResults
	details yodb.JsonMap[any]
	results yodb.JsonMap[any]
}

type TaskAttempt struct {
	Time time.Time
	Err  string
}

func (me *JobTask) State() RunState { return RunState(me.state) }

func (me *JobTask) Failed() bool {
	return (me.State() == Done) && (len(me.Attempts) > 0) && (me.Attempts[0].Err != "")
}

func (me *JobTask) Succeeded() bool {
	return (me.State() == Done) && (len(me.Attempts) > 0) && (me.Attempts[0].Err == "")
}

func (me *JobTask) markForRetryOrAsFailed(jobDef *JobDef) (retry bool) {
	if (jobDef != nil) && (len(me.Attempts) <= int(jobDef.MaxTaskRetries)) { // first attempt was not a RE-try
		me.state, me.StartTime, me.FinishTime = yodb.Text(Pending), nil, nil
		return true
	}
	me.state, me.FinishTime = yodb.Text(Done), yodb.DtNow()
	return false
}

// Timeout implements utils.HasTimeout
func (me *JobTask) Timeout(ctx *Ctx) time.Duration {
	var job_def *JobDef
	job_run := me.JobRun.Get(ctx)
	if job_run != nil {
		job_def = job_run.JobDef.Get(ctx)
	}
	if (job_def != nil) && (job_def.TimeoutTaskRunSecs) > 0 {
		return time.Second * time.Duration(job_def.TimeoutTaskRunSecs)
	}
	return TimeoutLong
}
