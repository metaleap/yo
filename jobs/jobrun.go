package yojobs

import (
	"time"

	. "yo/ctx"
	yodb "yo/db"
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
	Id      yodb.I64
	Version yodb.U32

	JobTypeId     yodb.Text
	JobDef        yodb.Ref[JobDef, yodb.RefOnDelSetNull]
	state         yodb.Text
	DueTime       *yodb.DateTime
	StartTime     *yodb.DateTime
	FinishTime    *yodb.DateTime
	AutoScheduled yodb.Bool

	// this is DB-uniqued and its only purpose is to avoid multiple instances concurrently scheduling the same next job in `ensureJobRunSchedules`
	ScheduledNextAfterJobRun yodb.Ref[JobRun, yodb.RefOnDelSetNull]

	InfoDurationPrepSecs     *float64
	InfoDurationFinalizeSecs *float64
	InfoCancellationReason   CancellationReason

	Details JobDetails
	Results JobResults
}

func (me *JobRun) State() RunState { return RunState(me.state) }

func (me *JobRun) ctx(ctx *Ctx, taskId string) *Context {
	return &Context{Ctx: ctx, JobRunId: me.Id, JobDetails: me.Details, JobDef: *me.JobDef.Get(ctx), JobTaskId: taskId}
}

type JobRunStats struct {
	TasksByState   map[RunState]int64
	TasksFailed    int64
	TasksSucceeded int64
	TasksTotal     int64

	DurationTotalMins    *float64
	DurationPrepSecs     *float64
	DurationFinalizeSecs *float64
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

// PercentSuccess returns a percentage `int` such that:
//   - 100 always means "job fully successful" (all its tasks succeeded),
//   - 0 always means "job fully failed" (all its tasks failed),
//   - 1-99 means a (technically slightly imprecise) approximation of the actual success/failure ratio,
//   - `nil` means the job is not yet `DONE`.
func (me *JobRunStats) PercentSuccess() *int {
	if me.TasksTotal == 0 || me.TasksByState[Done] != me.TasksTotal {
		return nil
	}
	switch me.TasksTotal {
	case me.TasksSucceeded:
		return ToPtr(100)
	case me.TasksFailed:
		return ToPtr(0)
	default:
		return ToPtr(Clamp(1, 99, int(float64(me.TasksSucceeded)*(100.0/float64(me.TasksTotal)))))
	}
}

// Timeout implements utils.HasTimeout
func (me *JobRun) Timeout(ctx *Ctx) time.Duration {
	job_def := me.JobDef.Get(ctx)
	if (job_def != nil) && (job_def.TimeoutJobRunPrepAndFinalizeSecs > 0) {
		return time.Second * time.Duration(job_def.TimeoutJobRunPrepAndFinalizeSecs)
	}
	return TimeoutLong
}
