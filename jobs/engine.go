package yojobs

import (
	"strconv"
	"sync"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	. "yo/util"
)

func init() {
	yodb.Ensure[JobDef, q.F]("", nil, false)
	yodb.Ensure[JobRun, q.F]("", nil, false)
	yodb.Ensure[JobTask, q.F]("", nil, false)
}

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

	// OnJobTaskExecuted takes an event handler (only one is kept) to be invoked when a task run has finished (successfully or not) and that run fully stored
	OnJobTaskExecuted(func(*JobTask, time.Duration))
	// OnJobRunFinalized takes an event handler (only one is kept) that should take care to `nil`-check its `JobRunStats` arg
	OnJobRunFinalized(func(*JobRun, *JobRunStats))
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

type engine struct {
	running          bool
	options          Options
	taskCancelers    map[string]func()
	taskCancelersMut sync.Mutex
	eventHandlers    struct {
		onJobTaskExecuted func(*JobTask, time.Duration)
		onJobRunFinalized func(*JobRun, *JobRunStats)
	}
}

func NewEngine(options Options) (Engine, error) {
	err := sanitizeOptionsFields[Options](2, 128, strconv.Atoi, map[string]*int{
		"MaxConcurrentOps": &options.MaxConcurrentOps,
		"FetchTasksToRun":  &options.FetchTasksToRun,
	})
	if err == nil {
		err = sanitizeOptionsFields[Options](2*time.Second, 22*time.Hour, time.ParseDuration, map[string]*time.Duration{
			"TimeoutShort":                     &options.TimeoutShort,
			"IntervalStartAndFinalizeJobs":     &options.IntervalStartAndFinalizeJobs,
			"IntervalRunTasks":                 &options.IntervalRunTasks,
			"IntervalExpireOrRetryDeadTasks":   &options.IntervalExpireOrRetryDeadTasks,
			"IntervalEnsureJobSchedules":       &options.IntervalEnsureJobSchedules,
			"IntervalDeleteStorageExpiredJobs": &options.IntervalDeleteStorageExpiredJobs,
		})
	}
	return nil, err // &engine{options: options, taskCancelers: map[string]func(){}}, nil
}

func (me *engine) Running() bool { return me.running }
func (me *engine) Resume() {
	me.running = true
	// DoAfter(it.options.IntervalStartAndFinalizeJobs, it.startAndFinalizeJobRuns)
	// DoAfter(it.options.IntervalRunTasks, it.runJobTasks)
	// DoAfter(it.options.IntervalExpireOrRetryDeadTasks, it.expireOrRetryDeadJobTasks)
	// DoAfter(it.options.IntervalDeleteStorageExpiredJobs/10, it.deleteStorageExpiredJobRuns)
	// DoAfter(Clamp(22*time.Second, 44*time.Second, it.options.IntervalEnsureJobSchedules), it.ensureJobRunSchedules)
}

func (me *engine) OnJobTaskExecuted(eventHandler func(*JobTask, time.Duration)) {
	me.eventHandlers.onJobTaskExecuted = eventHandler
}
func (me *engine) OnJobRunFinalized(eventHandler func(*JobRun, *JobRunStats)) {
	me.eventHandlers.onJobRunFinalized = eventHandler
}

// func (it *engine) cancelJobRuns(ctx context.Context, jobRunsToCancel map[CancellationReason][]*JobRun) (errs map[*JobRun]error) {
// 	if len(jobRunsToCancel) == 0 {
// 		return
// 	}
// 	log := loggerNew()
// 	var mut_errs sync.Mutex
// 	errs = make(map[*JobRun]error, len(jobRunsToCancel)/2)
// 	for reason, jobRuns := range jobRunsToCancel {
// 		GoItems(ctx, jobRuns, func(ctx context.Context, jobRun *JobRun) {
// 			state, version := jobRun.State, jobRun.Version
// 			jobRun.State, jobRun.Info.CancellationReason = JobRunCancelling, reason
// 			if it.logLifecycleEvents(nil, jobRun, nil) {
// 				jobRun.logger(log).Infof("marking %s '%s' job run '%s' as %s", state, jobRun.JobDefId, jobRun.Id, jobRun.State)
// 			}
// 			if err := it.storage.saveJobRun(ctx, jobRun); err != nil {
// 				jobRun.State, jobRun.Version = state, version
// 				mut_errs.Lock()
// 				errs[jobRun] = err
// 				mut_errs.Unlock()
// 			}
// 		}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
// 	}
// 	return
// }

func (it *engine) DeleteJobRuns(ctx *Ctx, jobRunIds ...yodb.I64) int64 {
	return yodb.Delete[JobRun](ctx, JobRunId.In(yodb.Arr[yodb.I64](jobRunIds).ToAnys()...).And(
		jobRunState.Equal(string(Done)).Or(jobRunState.Equal(string(Cancelled)))))
}

func (me *engine) Stats(ctx *Ctx, jobRunId yodb.I64) *JobRunStats {
	job_run := yodb.ById[JobRun](ctx, jobRunId)
	return job_run.Stats(ctx)
}

func (me *JobRun) Stats(ctx *Ctx) *JobRunStats {
	stats := JobRunStats{TasksByState: make(map[RunState]int64, 4)}

	for _, state := range []RunState{Pending, Running, Done, Cancelled} {
		stats.TasksByState[state] = yodb.Count[JobTask](ctx, JobTaskJobRun.Equal(me.Id).And(jobTaskState.Equal(string(state))), "", nil)
		stats.TasksTotal += stats.TasksByState[state]
	}
	stats.TasksFailed = yodb.Count[JobTask](ctx,
		JobTaskJobRun.Equal(me.Id).And(jobTaskState.Equal(string(Done))).And(JobTaskFailed.Equal(true)),
		"", nil)
	stats.TasksSucceeded = stats.TasksByState[Done] - stats.TasksFailed

	if (me.StartTime != nil) && (me.FinishTime != nil) {
		stats.DurationTotalMins = ToPtr(me.FinishTime.Sub(me.StartTime).Minutes())
	}
	stats.DurationPrepSecs, stats.DurationFinalizeSecs = me.DurationPrepSecs, me.DurationFinalizeSecs
	return &stats
}
