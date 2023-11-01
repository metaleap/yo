package jobs

import (
	"context"
	"time"

	. "yo/util"
)

type RunState string

const (
	RunStateUndefified RunState = ""
	Pending            RunState = "PENDING"
	Running            RunState = "RUNNING"
	Done               RunState = "DONE"
	Cancelled          RunState = "CANCELLED"
	// JobRunCancelling only exists for `JobRun`s, never for `JobTask`s.
	JobRunCancelling RunState = "CANCELLING"
)

type CancellationReason string

const (
	CancellationReasonDuplicate            CancellationReason = "JobRunDuplicate"
	CancellationReasonDefInvalidOrGone     CancellationReason = "JobDefInvalidOrGone"
	CancellationReasonDefChanged           CancellationReason = "JobDefChanged"
	CancellationReasonJobTypeInvalidOrGone CancellationReason = "JobTypeInvalidOrGone"
)

type JobRun struct {
	Id string

	JobTypeId     string
	JobDefId      string
	State         RunState
	DueTime       time.Time
	StartTime     *time.Time
	FinishTime    *time.Time
	AutoScheduled bool

	Details JobDetails `json:"-"`
	Results JobResults `json:"-"`
	// DetailsStore is for storage and not to be used in code outside internal un/marshaling hooks, use `Details`.
	DetailsStore map[string]any
	// ResultsStore is for storage and not to be used in code outside internal un/marshaling hooks, use `Results`.
	ResultsStore map[string]any

	// this is DB-uniqued and its only purpose is to avoid multiple instances concurrently scheduling the same next job in `ensureJobRunSchedules`
	ScheduledNextAfterJobRun string
	// FinalTaskFilter is obtained via call to JobType.TaskDetails() and stored for the later job finalization phase.
	FinalTaskFilter *JobTaskFilter
	// FinalTaskListReq is obtained via call to JobType.TaskDetails() and stored for the later job finalization phase.
	FinalTaskListReq *ListRequest

	Info struct { // Informational purposes only
		DurationPrepSecs     *float64
		DurationFinalizeSecs *float64
		CancellationReason   CancellationReason
	}

	Version int

	jobDef *JobDef
}

func (it *JobRun) ctx(ctx context.Context, taskId string) *Context {
	return &Context{Context: ctx, JobRunId: it.Id, JobDetails: it.Details, JobDef: *it.jobDef, JobTaskId: taskId}
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
func (it *JobRunStats) PercentDone() int {
	switch it.TasksTotal {
	case 0, (it.TasksByState[Pending] + it.TasksByState[Running]):
		return 0
	case (it.TasksByState[Done] + it.TasksByState[Cancelled]):
		return 100
	default:
		return Clamp(1, 99, int(float64(it.TasksByState[Done]+it.TasksByState[Cancelled])*(100.0/float64(it.TasksTotal))))
	}
}

// PercentSuccess returns a percentage `int` such that:
//   - 100 always means "job fully successful" (all its tasks succeeded),
//   - 0 always means "job fully failed" (all its tasks failed),
//   - 1-99 means a (technically slightly imprecise) approximation of the actual success/failure ratio,
//   - `nil` means the job is not yet `DONE`.
func (it *JobRunStats) PercentSuccess() *int {
	if it.TasksTotal == 0 || it.TasksByState[Done] != it.TasksTotal {
		return nil
	}
	switch it.TasksTotal {
	case it.TasksSucceeded:
		return ToPtr(100)
	case it.TasksFailed:
		return ToPtr(0)
	default:
		return ToPtr(Clamp(1, 99, int(float64(it.TasksSucceeded)*(100.0/float64(it.TasksTotal)))))
	}
}

// Timeout implements utils.HasTimeout
func (it *JobRun) Timeout() time.Duration {
	if (it.jobDef != nil) && (it.jobDef.Timeouts.JobRunPrepAndFinalize > 0) {
		return it.jobDef.Timeouts.JobRunPrepAndFinalize
	}
	return TimeoutLong
}
