package yojobs

import (
	"sync"
	"time"

	. "yo/ctx"
)

// TimeoutLong is:
//   - the default fallback for `JobDef`s without a custom `Timeouts.TaskRun`.
//   - the default fallback for `JobDef`s without a custom `Timeouts.JobPrepAndFinalize`.
//   - the timeout active during multiple task cancellations. Left-overs are still picked up by a follow-up cancellation-finalizer.
const TimeoutLong = 2 * time.Minute

// All times handled, stored, loaded etc. are relocated into this `timezone`.
// Only set it once no later than `init` time (if at all), and have it be the same every time across restarts (not a candidate for configurability).
var Timezone = time.UTC

type Engine interface {
	// Resume starts the `Engine`, ie. its (from then on) regularly-recurring background watchers.
	// nb. no `Suspend()` yet, currently out of scope.
	Resume()
	// Running returns `true` after `Resume` was called and `false` until then.
	Running() bool
	// CreateJobRun "manually schedules" an off-schedule job at the defified `dueTime`, which if missing (or set in the past) defaults to `timeNow()`.
	CreateJobRun(ctx *Ctx, jobDef *JobDef, jobRunId string, dueTime *time.Time, details JobDetails) (jobRun *JobRun, err error)
	// CancelJobRuns marks the defified jobs as `CANCELLING`. The `len(errs)` is always `<= len(jobIDs)`.
	CancelJobRuns(ctx *Ctx, jobRunIds ...string) (errs []error)
	// DeleteJobRun clears from storage the defified DONE or CANCELLED `JobRun` and all its `JobTask`s, if any.
	DeleteJobRuns(ctx *Ctx, jobRunIds ...string) error
	// Stats gathers progress stats of a `JobRun` and its `JobTask`s.
	Stats(ctx *Ctx, jobRunId string) (*JobRunStats, error)
	// RetryJobTask retries the defified failed task.
	RetryJobTask(ctx *Ctx, jobRunId string, jobTaskId string) (*JobTask, error)

	// OnJobTaskExecuted takes an event handler (only one is kept) to be invoked when a task run has been run (successfully or not) and that run stored
	OnJobTaskExecuted(func(*JobTask, time.Duration))
	// OnJobRunFinalized takes an event handler (only one is kept) that should take care to `nil`-check its `JobRunStats` arg
	OnJobRunFinalized(func(*JobRun, *JobRunStats))
}

type engine struct {
	running       bool
	options       Options
	taskCancelers struct {
		cache map[string]func()
		mut   sync.Mutex
	}
	eventHandlers struct {
		onJobTaskExecuted func(*JobTask, time.Duration)
		onJobRunFinalized func(*JobRun, *JobRunStats)
	}
}

type Options struct {
	// LogJobLifecycleEvents enables the info-level logging of every attempted RunState transition of all `JobRun`s.
	LogJobLifecycleEvents bool
	// LogTaskLifecycleEvents enables the info-level logging of every attempted RunState transition of all `JobTask`s.
	LogTaskLifecycleEvents bool

	// these "intervals" aren't "every n, do" but more like "wait n to go again _after_ done"

	// IntervalStartAndFinalizeJobs should be under 0.5 minutes.
	IntervalStartAndFinalizeJobs time.Duration `default:"22s"`
	// IntervalRunTasks should be under 0.5 minutes.
	IntervalRunTasks time.Duration `default:"11s"`
	// IntervalExpireOrRetryDeadTasks is advised every couple of minutes (under 5). It ensures (in storage) retry-or-done-with-error of tasks whose last runner died between their completion and updating their Result and RunState in storage accordingly.
	IntervalExpireOrRetryDeadTasks time.Duration `default:"3m"`
	// IntervalEnsureJobSchedules is advised every couple of minutes (under 5). It is only there to catch up scheduling-wise with new or changed `JobDef`s; otherwise a finalized `JobRun` gets its next occurrence scheduled right at finalization.
	IntervalEnsureJobSchedules time.Duration `default:"2m"`
	// IntervalDeleteStorageExpiredJobs can be on the order of hours: job storage-expiry is set in number-of-days.
	// However, a fluke failure will not see immediate retries (since running on an interval anyway), so no need to stretch too long either.
	IntervalDeleteStorageExpiredJobs time.Duration `default:"11h"`

	// MaxConcurrentOps semaphores worker bulk operations over multiple unrelated JobTasks, JobRuns or JobDefs
	MaxConcurrentOps int `default:"6"`
	// FetchTasksToRun denotes the maximum number of tasks-to-run-now to fetch, approx. every `IntervalRunTasks`.
	FetchTasksToRun int `default:"3"`
	// TimeoutShort is the usual timeout for most timeoutable calls (ie. brief DB queries and simple non-batch, non-transaction updates).
	// It should be well under 1min, and is not applicable for the cases described for `const TimeoutLong`.
	TimeoutShort time.Duration `default:"22s"`
}
