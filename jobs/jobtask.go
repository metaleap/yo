package yojobs

import (
	"errors"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	yojson "yo/json"
	. "yo/util"
	"yo/util/dict"
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

func (me *JobTask) markForRetryOrAsFailed(ctx *Ctx) (retry bool) {
	job_def := me.jobDef(ctx)
	if (job_def != nil) && (len(me.Attempts) <= int(job_def.MaxTaskRetries)) { // `<=` because first attempt was not a RE-try
		me.state, me.StartTime, me.FinishTime = yodb.Text(Pending), nil, nil
		return true
	}
	me.state, me.FinishTime = yodb.Text(Done), yodb.DtNow()
	return false
}

func (me *JobTask) jobRun(ctx *Ctx) *JobRun {
	if me != nil {
		return Cache(ctx, me.JobRun.Id(), func() *JobRun { return me.JobRun.Get(ctx) })
	}
	return nil
}

func (me *JobTask) jobDef(ctx *Ctx) *JobDef {
	return me.jobRun(ctx).jobDef(ctx)
}

func (me *JobTask) jobType(ctx *Ctx) JobType {
	if jobdef := me.jobDef(ctx); jobdef != nil {
		return jobdef.jobType
	}
	return nil
}

var _ yodb.Obj = (*JobTask)(nil)     // compile-time interface compat check
var _ jobRunOrTask = (*JobTask)(nil) // dito

func (me *JobTask) id() yodb.I64 { return me.Id }
func (me *JobTask) version(newVersion yodb.U32) yodb.U32 {
	if newVersion > 0 {
		me.Version = newVersion
	}
	return me.Version
}

func (me *JobTask) OnAfterLoaded() { // any changes, keep in sync with JobRun.OnAfterLoaded
	job_type_reg := jobType(string(me.JobTypeId))
	if job_type_reg != nil {
		me.Details, me.Results = job_type_reg.loadTaskDetails(me.details), job_type_reg.loadTaskResults(me.results)
	}
}
func (me *JobTask) OnBeforeStoring(isCreate bool) (q.Query, []q.F) { // any changes, keep in sync with JobRun.OnBeforeStoring
	me.details, me.results = yojson.DictFrom(me.Details), yojson.DictFrom(me.Results)
	old_version := me.Version
	if (!isCreate) && (old_version <= 0) {
		panic("Update of JobTask without current version")
	}
	me.Version++
	return JobTaskVersion.Equal(old_version), JobTaskFields(JobTaskVersion)
}

type TaskAttempt struct {
	T   string
	Err error

	t *time.Time
}

func (me *TaskAttempt) UnmarshalJSON(json_src []byte) error {
	obj := dict.Any{}
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
	obj := dict.Any{}
	if me.Err != nil { // want this first in the DB (only if err tho), handy for occasional manual sorting in local dev among many 1000s of tasks
		obj["e"] = me.Err.Error()
	}
	obj["t"] = me.T
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
