package yojobs

import (
	"time"

	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	yojson "yo/json"
	. "yo/util"
)

type RunState string

const (
	Pending   RunState = "PENDING"
	Running   RunState = "RUNNING"
	Done      RunState = "DONE"
	Cancelled RunState = "CANCELLED"
	// JobRunCancelling only exists for `JobRun`s, never for `JobTask`s.
	JobRunCancelling RunState = "CANCELLING"
)

type CancellationReason string

const (
	CancellationReasonJobRunDuplicate      CancellationReason = "JobRunDuplicate"
	CancellationReasonJobDefInvalidOrGone  CancellationReason = "JobDefInvalidOrGone"
	CancellationReasonJobDefChanged        CancellationReason = "JobDefChanged"
	CancellationReasonJobTypeInvalidOrGone CancellationReason = "JobTypeInvalidOrGone"
)

type JobRun struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	Version            yodb.U32
	JobTypeId          yodb.Text
	JobDef             yodb.Ref[JobDef, yodb.RefOnDelSetNull]
	state              yodb.Text
	cancellationReason yodb.Text
	DueTime            *yodb.DateTime
	StartTime          *yodb.DateTime
	FinishTime         *yodb.DateTime
	AutoScheduled      yodb.Bool

	// this is DB-uniqued and its only purpose is to avoid multiple instances concurrently scheduling the same next job in `ensureJobRunSchedules`
	ScheduledNextAfter yodb.Ref[JobRun, yodb.RefOnDelSetNull]

	DurationPrepSecs     yodb.F32
	DurationFinalizeSecs yodb.F32

	Details JobDetails
	Results JobResults
	details yodb.JsonMap[any]
	results yodb.JsonMap[any]
}

func (me *JobRun) State() RunState { return RunState(me.state) }
func (me *JobRun) CancellationReason() CancellationReason {
	return CancellationReason(me.cancellationReason)
}

func (me *JobRun) ctx(ctx *Ctx, taskId yodb.I64) *Ctx {
	return ctx.WithJob(int64(me.Id), me.Details, int64(taskId))
}

type JobRunStats struct {
	TasksByState map[RunState]int64
	TasksTotal   int64

	DurationTotalMins    *float64
	DurationPrepSecs     *yodb.F32
	DurationFinalizeSecs *yodb.F32
}

// PercentDone returns a percentage `int` such that:
//   - 100 always means all tasks are DONE or CANCELLED,
//   - 0 always means no tasks are DONE or CANCELLED (or none exist yet),
//   - 1-99 means a (technically slightly imprecise) approximation of the actual ratio.
func (me *JobRunStats) PercentDone() int {
	switch me.TasksTotal {
	case 0, (me.TasksByState[Pending] + me.TasksByState[Running]):
		return 0
	case (me.TasksByState[Done] + me.TasksByState[Cancelled]):
		return 100
	default:
		return Clamp(1, 99, int(float64(me.TasksByState[Done]+me.TasksByState[Cancelled])*(100.0/float64(me.TasksTotal))))
	}
}

func (me *JobRun) TimeoutPrepAndFinalize(ctx *Ctx) time.Duration {
	job_def := me.jobDef(ctx)
	if (job_def != nil) && (job_def.TimeoutSecsJobRunPrepAndFinalize > 0) {
		return time.Second * time.Duration(job_def.TimeoutSecsJobRunPrepAndFinalize)
	}
	return TimeoutLong
}

func (me *JobRun) Stats(ctx *Ctx) *JobRunStats {
	ctx.DbTx()
	stats := JobRunStats{TasksByState: make(map[RunState]int64, 4)}

	for _, state := range []RunState{Pending, Running, Done, Cancelled} {
		stats.TasksByState[state] = yodb.Count[JobTask](ctx, JobTaskJobRun.Equal(me.Id).And(jobTaskState.Equal(string(state))), "", nil)
		stats.TasksTotal += stats.TasksByState[state]
	}

	if (me.StartTime != nil) && (me.FinishTime != nil) {
		stats.DurationTotalMins = ToPtr(me.FinishTime.Sub(me.StartTime).Minutes())
	}
	if me.DurationPrepSecs != 0 {
		stats.DurationPrepSecs = &me.DurationPrepSecs
	}
	if me.DurationFinalizeSecs != 0 {
		stats.DurationFinalizeSecs = &me.DurationFinalizeSecs
	}
	return &stats
}

func (me *JobRun) jobDef(ctx *Ctx) *JobDef {
	if me != nil {
		return Cache(ctx, me.JobDef.Id(), func() *JobDef { return me.JobDef.Get(ctx) })
	}
	return nil
}

func (me *JobRun) jobType(ctx *Ctx) JobType {
	if jobdef := me.jobDef(ctx); jobdef != nil {
		return jobdef.jobType
	}
	return nil
}

var _ yodb.Obj = (*JobRun)(nil)     // compile-time interface compat check
var _ jobRunOrTask = (*JobRun)(nil) // dito

func (me *JobRun) id() yodb.I64 { return me.Id }
func (me *JobRun) version(newVersion yodb.U32) yodb.U32 {
	if newVersion > 0 {
		me.Version = newVersion
	}
	return me.Version
}

func (me *JobRun) OnAfterLoaded() { // any changes, keep in sync with JobTask.OnAfterLoaded
	job_type_reg := jobType(string(me.JobTypeId))
	if job_type_reg != nil {
		me.Details, me.Results = job_type_reg.loadJobDetails(me.details), job_type_reg.loadJobResults(me.results)
	}
}
func (me *JobRun) OnBeforeStoring(isCreate bool) (q.Query, []q.F) { // any changes, keep in sync with JobTask.OnBeforeStoring
	me.details, me.results = yojson.DictFrom(me.Details), yojson.DictFrom(me.Results)
	old_version := me.Version
	if (!isCreate) && (old_version <= 0) {
		panic("Update of JobRun without current version")
	}
	me.Version++
	return JobRunVersion.Equal(old_version), JobRunFields(JobRunVersion)
}
