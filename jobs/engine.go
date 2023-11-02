package yojobs

import (
	"context"
	"strconv"
	"sync"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	. "yo/util"
	sl "yo/util/sl"
)

func init() {
	yodb.Ensure[JobDef, q.F]("", nil, false)
	yodb.Ensure[JobRun, q.F]("", nil, false, yodb.Unique[JobRunField]{JobRunScheduledNextAfter})
	yodb.Ensure[JobTask, q.F]("", nil, false)
}

// TimeoutLong is:
//   - the default fallback for `JobDef`s without a custom `Timeouts.TaskRun`.
//   - the default fallback for `JobDef`s without a custom `Timeouts.JobPrepAndFinalize`.
//   - the timeout active during multiple task cancellations. Left-overs are still picked up by a follow-up cancellation-finalizer.
const TimeoutLong = 2 * time.Minute

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
	taskCancelers    map[yodb.I64]func()
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
	if me.running {
		return
	}
	me.running = true
	// DoAfter(me.options.IntervalStartAndFinalizeJobs, me.startAndFinalizeJobRuns)
	DoAfter(me.options.IntervalRunTasks, me.runJobTasks)
	// DoAfter(me.options.IntervalExpireOrRetryDeadTasks, me.expireOrRetryDeadJobTasks)
	// DoAfter(me.options.IntervalDeleteStorageExpiredJobs/10, me.deleteStorageExpiredJobRuns)
	// DoAfter(Clamp(22*time.Second, 44*time.Second, me.options.IntervalEnsureJobSchedules), me.ensureJobRunSchedules)
}

func (me *engine) OnJobTaskExecuted(eventHandler func(*JobTask, time.Duration)) {
	me.eventHandlers.onJobTaskExecuted = eventHandler
}
func (me *engine) OnJobRunFinalized(eventHandler func(*JobRun, *JobRunStats)) {
	me.eventHandlers.onJobRunFinalized = eventHandler
}

func (*engine) CancelJobRuns(ctx *Ctx, jobRunIds ...string) {
	if len(jobRunIds) == 0 {
		return
	}
	yodb.Update[JobRun](ctx, &JobRun{state: yodb.Text(JobRunCancelling)},
		JobRunId.In(yodb.Arr[string](jobRunIds).ToAnys()...), true, JobRunFields(jobRunState)...)
}

func (*engine) cancelJobRuns(ctx *Ctx, jobRunsToCancel map[CancellationReason][]*JobRun) {
	if len(jobRunsToCancel) == 0 {
		return
	}
	ctx.DbTx()
	for reason, job_runs := range jobRunsToCancel {
		var failed bool
		Try(func() {
			yodb.Update[JobRun](ctx, &JobRun{cancellationReason: yodb.Text(reason), state: yodb.Text(JobRunCancelling)},
				JobRunId.In(sl.To(job_runs, func(it *JobRun) any { return it.Id })), true, JobRunFields(jobRunState, jobRunCancellationReason)...)
		}, func(any) { failed = true })
		if !failed {
			for _, job_run := range job_runs {
				job_run.state, job_run.cancellationReason = yodb.Text(JobRunCancelling), yodb.Text(reason)
			}
		}
	}
}

func (me *engine) CreateJobRun(ctx *Ctx, jobDef *JobDef, dueTime *yodb.DateTime, jobDetails JobDetails) *JobRun {
	if now := yodb.DtNow(); (dueTime == nil) || now.Time().After(*dueTime.Time()) {
		dueTime = now
	}
	return me.createJobRun(ctx, jobDef, dueTime, jobDetails, nil)
}

func (*engine) createJobRun(ctx *Ctx, jobDef *JobDef, dueTime *yodb.DateTime, jobDetails JobDetails, autoScheduledNextAfter *JobRun) *JobRun {
	is_auto_scheduled := yodb.Bool(autoScheduledNextAfter != nil)
	if jobDef.Disabled || ((!jobDef.AllowManualJobRuns) && !is_auto_scheduled) {
		return nil
	}
	ctx.DbTx()
	job_run := &JobRun{
		state:         yodb.Text(Pending),
		Details:       jobDetails,
		JobTypeId:     jobDef.JobTypeId,
		DueTime:       dueTime,
		AutoScheduled: is_auto_scheduled,
	}
	job_run.JobDef.SetId(jobDef.Id)
	if is_auto_scheduled {
		job_run.ScheduledNextAfter.SetId(autoScheduledNextAfter.Id)
		if yodb.Exists[JobRun](ctx, JobRunScheduledNextAfter.Equal(autoScheduledNextAfter.Id)) {
			return nil // this above check by design as-late-as-possible before our own below Create (db-uniqued anyway, but nice to avoid the bound-to-fail attempt without erroring)
		}
	}
	job_run.Id = yodb.CreateOne[JobRun](ctx, job_run)
	return job_run
}

func (*engine) DeleteJobRuns(ctx *Ctx, jobRunIds ...yodb.I64) int64 {
	return yodb.Delete[JobRun](ctx, JobRunId.In(yodb.Arr[yodb.I64](jobRunIds).ToAnys()...).And(
		jobRunState.Equal(string(Done)).Or(jobRunState.Equal(string(Cancelled)))))
}

func (me *JobTask) Retry(ctx *Ctx) {
	ctx.DbTx()
}

func (*engine) Stats(ctx *Ctx, jobRunId yodb.I64) *JobRunStats {
	ctx.DbTx()
	job_run := yodb.ById[JobRun](ctx, jobRunId)
	return job_run.Stats(ctx)
}

func (me *engine) setTaskCanceler(taskId yodb.I64, cancel context.CancelFunc) (previous context.CancelFunc) {
	me.taskCancelersMut.Lock()
	defer me.taskCancelersMut.Unlock()
	previous = me.taskCancelers[taskId]
	if cancel == nil {
		delete(me.taskCancelers, taskId)
	} else {
		me.taskCancelers[taskId] = cancel
	}
	return
}
