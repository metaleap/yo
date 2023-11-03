package yojobs

import (
	"strconv"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	. "yo/util"
	sl "yo/util/sl"
)

func init() {
	yodb.Ensure[JobDef, JobDefField]("", nil, false,
		yodb.Unique[JobDefField]{JobDefName})
	yodb.Ensure[JobRun, JobRunField]("", nil, false,
		yodb.Unique[JobRunField]{JobRunScheduledNextAfter},
		yodb.AlwaysFetch[JobRunField]{JobRunVersion},
		yodb.Index[JobRunField]{jobRunState})
	yodb.Ensure[JobTask, JobTaskField]("", nil, false,
		yodb.AlwaysFetch[JobTaskField]{JobTaskVersion},
		yodb.Index[JobTaskField]{jobTaskState})
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
	// CreateJobRun "manually schedules" an off-schedule job at the specified `dueTime`, which if missing (or set in the past) defaults to `timeNow()`.
	CreateJobRun(ctx *Ctx, jobDef *JobDef, dueTime *yodb.DateTime) *JobRun
	// CancelJobRuns marks the specified jobs as `CANCELLING`. The `len(errs)` is always `<= len(jobIDs)`.
	CancelJobRuns(ctx *Ctx, jobRunIds ...string)
	// DeleteJobRun clears from storage the specified DONE or CANCELLED `JobRun` and all its `JobTask`s, if any.
	DeleteJobRuns(ctx *Ctx, jobRunIds ...yodb.I64) int64
	// Stats gathers progress stats of a `JobRun` and its `JobTask`s.
	Stats(ctx *Ctx, jobRunId yodb.I64) *JobRunStats

	// OnJobTaskExecuted takes an event handler (only one is kept) to be invoked when a task run has finished (successfully or not) and that run fully stored
	OnJobTaskExecuted(func(*JobTask, time.Duration))
	// OnJobRunFinalized takes an event handler (only one is kept) that should take care to `nil`-check its `JobRunStats` arg
	OnJobRunFinalized(func(*JobRun, *JobRunStats))
}

type Options struct {
	// IntervalStartAndFinalizeJobs should be under 0.5 minutes.
	IntervalStartAndFinalizeJobs time.Duration `default:"11s"`
	// IntervalRunTasks should be under 0.5 minutes.
	IntervalRunTasks time.Duration `default:"11s"`
	// IntervalExpireOrRetryDeadTasks is advised every couple of minutes (under 5). It ensures (in storage) retry-or-done-with-error of tasks whose last runner died between their completion and updating their Result and RunState in storage accordingly.
	IntervalExpireOrRetryDeadTasks time.Duration `default:"2m"`
	// IntervalEnsureJobSchedules is advised every couple of minutes (under 5). It is only there to catch up scheduling-wise with new or changed `JobDef`s; otherwise a finalized `JobRun` gets its next occurrence scheduled right at finalization.
	IntervalEnsureJobSchedules time.Duration `default:"1m"`
	// IntervalDeleteStorageExpiredJobs can be on the order of hours: job storage-expiry is set in number-of-days.
	// However, a fluke failure (connectivity/DB-restart/etc) will not see immediate retries (since running on an interval anyway), so no need to stretch too long either.
	IntervalDeleteStorageExpiredJobs time.Duration `default:"11h"`

	// MaxConcurrentOps semaphores worker bulk operations over multiple unrelated JobTasks, JobRuns or JobDefs.
	// keep it lowish since importers are also serving api/asset requests and many such bulk-operations might incur DB table-locks (or db driver locks) anyway
	MaxConcurrentOps int `default:"4"`
	// FetchTasksToRun denotes the maximum number of tasks-to-run-now to fetch, approx. every `IntervalRunTasks`.
	FetchTasksToRun int `default:"22"`
	// TimeoutShort is the usual timeout for most timeoutable calls (ie. brief DB queries and simple non-batch, non-transaction updates).
	// It should be well under 1min, and is not applicable for the cases described for `const TimeoutLong`.
	TimeoutShort time.Duration `default:"22s"`
}

type engine struct {
	running       bool
	options       Options
	eventHandlers struct {
		onJobTaskExecuted func(*JobTask, time.Duration)
		onJobRunFinalized func(*JobRun, *JobRunStats)
	}
}

func NewEngine(options Options) Engine {
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
	if err != nil {
		panic(err)
	}
	return &engine{options: options}
}

func (me *engine) Running() bool { return me.running }
func (me *engine) Resume() {
	if me.running {
		return
	}
	me.running = true
	// why the `Clamp`s: no matter the intervals for future iterations, the very first call per worker upon (re)start should be "soon-ish"
	DoAfter(Clamp(time.Second, time.Minute, me.options.IntervalStartAndFinalizeJobs), me.startAndFinalizeJobRuns)
	DoAfter(Clamp(time.Second, time.Minute, me.options.IntervalRunTasks), me.runJobTasks)
	DoAfter(Clamp(time.Second, time.Minute, me.options.IntervalExpireOrRetryDeadTasks), me.expireOrRetryDeadJobTasks)
	DoAfter(Clamp(time.Second, time.Minute, me.options.IntervalDeleteStorageExpiredJobs)/10, me.deleteStorageExpiredJobRuns)
	DoAfter(Clamp(time.Second, time.Minute, me.options.IntervalEnsureJobSchedules), me.ensureJobRunSchedules)
}

func (me *engine) OnJobTaskExecuted(eventHandler func(*JobTask, time.Duration)) {
	me.eventHandlers.onJobTaskExecuted = eventHandler
}
func (me *engine) OnJobRunFinalized(eventHandler func(*JobRun, *JobRunStats)) {
	me.eventHandlers.onJobRunFinalized = eventHandler
}

func (me *engine) CancelJobRuns(ctx *Ctx, jobRunIds ...string) {
	me.cancelJobRuns(ctx, map[CancellationReason][]*JobRun{
		"": yodb.FindMany[JobRun](ctx, JobRunId.In(sl.Of[string](jobRunIds).ToAnys()...).And(jobRunState.In(Running, Pending)), 0, JobRunFields(JobRunId)),
	})
}

func (me *engine) cancelJobRuns(ctx *Ctx, jobRunsToCancel map[CancellationReason][]*JobRun) {
	if len(jobRunsToCancel) == 0 {
		return
	}
	for reason, job_runs := range jobRunsToCancel {
		Try(func() {
			dbBatchUpdate(me, ctx, job_runs, &JobRun{state: yodb.Text(JobRunCancelling), cancellationReason: yodb.Text(reason)}, JobRunFields(jobRunState, jobRunCancellationReason)...)
		}, nil)
	}
}

func (me *engine) CreateJobRun(ctx *Ctx, jobDef *JobDef, dueTime *yodb.DateTime) *JobRun {
	if now := yodb.DtNow(); (dueTime == nil) || now.Time().After(*dueTime.Time()) {
		dueTime = now
	}
	return me.createJobRun(ctx, jobDef, dueTime, nil, false)
}

func (*engine) createJobRun(ctx *Ctx, jobDef *JobDef, dueTime *yodb.DateTime, autoScheduledNextAfter *JobRun, isAutoScheduled yodb.Bool) *JobRun {
	if jobDef.Disabled || ((!jobDef.AllowManualJobRuns) && !isAutoScheduled) {
		return nil
	}
	ctx.DbTx()
	job_run := &JobRun{
		state:         yodb.Text(Pending),
		JobTypeId:     jobDef.JobTypeId,
		DueTime:       dueTime,
		AutoScheduled: isAutoScheduled, // need this extra bool arg in case `autoScheduledNextAfter` is nil for the very first auto-scheduling
	}
	job_run.JobDef.SetId(jobDef.Id)
	if autoScheduledNextAfter != nil {
		job_run.ScheduledNextAfter.SetId(autoScheduledNextAfter.Id)
		if yodb.Exists[JobRun](ctx, JobRunScheduledNextAfter.Equal(autoScheduledNextAfter.Id)) {
			return nil // this above check by design as-late-as-possible before our own below Create (db-uniqued anyway, but nice to avoid the bound-to-fail attempt without erroring)
		}
	}
	job_run.Id = yodb.CreateOne[JobRun](ctx, job_run)
	return job_run
}

func (*engine) DeleteJobRuns(ctx *Ctx, jobRunIds ...yodb.I64) int64 {
	return yodb.Delete[JobRun](ctx, JobRunId.In(sl.Of[yodb.I64](jobRunIds).ToAnys()...).And(
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

func dbBatchUpdate[TObj any](me *engine, ctx *Ctx, objs []*TObj, upd *TObj, onlyFields ...q.F) {
	type task_or_job = interface{ id() yodb.I64 }
	GoItems(objs, func(obj *TObj) {
		ctx := ctx.CopyButWith(0, false)
		defer ctx.OnDone(nil)
		yodb.Update[TObj](ctx, upd, yodb.ColID.Equal(any(obj).(task_or_job).id()), false, onlyFields...)
	}, me.options.MaxConcurrentOps)
}
