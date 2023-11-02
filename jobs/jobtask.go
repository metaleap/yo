package yojobs

import (
	"errors"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	yojson "yo/json"
	. "yo/util"
)

type JobTask struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	Version    yodb.U32
	JobTypeId  yodb.Text
	JobRun     yodb.Ref[JobRun, yodb.RefOnDelCascade]
	state      yodb.Text
	StartTime  *yodb.DateTime
	FinishTime *yodb.DateTime
	Attempts   yodb.JsonArr[TaskAttempt]
	Failed     yodb.Bool

	Details TaskDetails
	Results TaskResults
	details yodb.JsonMap[any]
	results yodb.JsonMap[any]
}

func (me *JobTask) State() RunState { return RunState(me.state) }

func (me *JobTask) failed() bool {
	return (me.State() == Done) && (len(me.Attempts) > 0) && (me.Attempts[0].Err != nil)
}

func (me *JobTask) Succeeded() bool {
	return (me.State() == Done) && (len(me.Attempts) > 0) && (me.Attempts[0].Err == nil)
}

func (me *JobTask) markForRetryOrAsFailed(jobDef *JobDef) (retry bool) {
	if (jobDef != nil) && (len(me.Attempts) <= int(jobDef.MaxTaskRetries)) { // `<=` because first attempt was not a RE-try
		me.state, me.StartTime, me.FinishTime = yodb.Text(Pending), nil, nil
		return true
	}
	me.state, me.FinishTime, me.Failed = yodb.Text(Done), yodb.DtNow(), true
	return false
}

// Timeout implements utils.HasTimeout
func (me *JobTask) Timeout(ctx *Ctx) time.Duration {
	job_def := me.jobDef(ctx)
	if (job_def != nil) && (job_def.TimeoutSecsTaskRun) > 0 {
		return time.Second * time.Duration(job_def.TimeoutSecsTaskRun)
	}
	return TimeoutLong
}

func (me *JobTask) jobRun(ctx *Ctx) *JobRun {
	if me != nil {
		return me.JobRun.Get(ctx)
	}
	return nil
}

func (me *JobTask) jobDef(ctx *Ctx) *JobDef {
	return me.jobRun(ctx).jobDef(ctx)
}

type TaskAttempt struct {
	T   string
	Err error

	t *time.Time
}

func (me *TaskAttempt) UnmarshalJSON(json_src []byte) error {
	obj := map[string]any{}
	yojson.Load(json_src, &obj)
	str_err, _ := obj["e"].(string)
	str_dt, _ := obj["t"].(string)
	me.Err = If(str_err == "", nil, errors.New(str_err))
	t, err := time.Parse(time.RFC3339, str_dt)
	if me.T, me.t = "", If((str_dt == "") || (err != nil), nil, &t); me.t != nil {
		me.T = me.t.Format(time.RFC3339)
	}
	return nil
}

func (me *TaskAttempt) MarshalJSON() ([]byte, error) {
	obj := map[string]any{"t": me.T}
	if me.Err != nil {
		obj["e"] = me.Err.Error()
	}
	return yojson.From(obj, false), nil
}

func taskAttempt() TaskAttempt {
	t := ToPtr(time.Now())
	return TaskAttempt{T: t.Format(time.RFC3339), t: t}
}

func (me *TaskAttempt) Time() *time.Time {
	if (me.t == nil) && (me.T != "") {
		if t, err := time.Parse(time.RFC3339, me.T); err == nil {
			me.t = &t
		}
	}
	return me.t
}
