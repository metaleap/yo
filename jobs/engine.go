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

var Default = NewEngine(Options{})

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

// Timeout1Min is:
//   - the default fallback for `JobDef`s without a custom `Timeouts.TaskRun`.
//   - the default fallback for `JobDef`s without a custom `Timeouts.JobPrepAndFinalize`.
//   - the timeout for the workers scheduled in `Engine.Resume` (further below)
const Timeout1Min = time.Minute

type Engine interface {
	// Resume starts the `Engine`, ie. its (from then on) regularly-recurring background workers.
	Resume()
	// Running returns `true` after `Resume` was called and `false` until then.
	Running() bool
	// CreateJobRun "manually schedules" an off-schedule job at the specified `dueTime`, which if missing (or set in the past) defaults to `timeNow()`.
	CreateJobRun(ctx *Ctx, jobDef *JobDef, dueTime *yodb.DateTime) *JobRun
	// DeleteJobRun clears from storage the specified DONE or CANCELLED `JobRun` and all its `JobTask`s, if any.
	DeleteJobRuns(ctx *Ctx, jobRunIds ...yodb.I64) int64
	// Stats gathers progress stats of a `JobRun` and its `JobTask`s.
	Stats(ctx *Ctx, jobRunId yodb.I64) *JobRunStats
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
	FetchTasksToRun int `default:"44"`
}

type engine struct {
	running bool
	options Options
}

func NewEngine(options Options) Engine {
	err := sanitizeOptionsFields[Options](2, 128, strconv.Atoi, map[string]*int{
		"MaxConcurrentOps": &options.MaxConcurrentOps,
		"FetchTasksToRun":  &options.FetchTasksToRun,
	})
	if err == nil {
		err = sanitizeOptionsFields[Options](2*time.Second, 22*time.Hour, time.ParseDuration, map[string]*time.Duration{
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
	DoAfter(1*time.Second, me.startAndFinalizeJobRuns)
	DoAfter(2*time.Second, me.runJobTasks)
	DoAfter(3*time.Second, me.ensureJobRunSchedules)
	DoAfter(4*time.Second, me.expireOrRetryDeadJobTasks)
	DoAfter(5*time.Second, me.deleteStorageExpiredJobRuns)
}

func (me *engine) cancelJobRuns(ctx *Ctx, jobRunsToCancel map[CancellationReason][]*JobRun) {
	if len(jobRunsToCancel) == 0 {
		return
	}
	for reason, job_runs := range jobRunsToCancel {
		dbBatchUpdate(me, ctx, job_runs, &JobRun{state: yodb.Text(JobRunCancelling), CancelReason: yodb.Text(reason)}, JobRunFields(jobRunState, JobRunCancelReason)...)
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

type jobRunOrTask = interface {
	version(yodb.U32) yodb.U32
	id() yodb.I64
}

func dbBatchUpdate[TObj any](me *engine, ctx *Ctx, objs []*TObj, upd *TObj, onlyFields ...q.F) {
	GoItems(objs, func(obj *TObj) {
		job_or_task := any(obj).(jobRunOrTask)
		any(upd).(jobRunOrTask).version(job_or_task.version(0))
		yodb.Update[TObj](ctx, upd, yodb.ColID.Equal(job_or_task.id()), false, onlyFields...)
	}, me.options.MaxConcurrentOps)
}

func Init(ctx *Ctx) {
	job_ids_to_delete := yodb.Ids[JobRun](ctx, JobRunCancelReason.Equal(CancellationReasonJobDefInvalidOrGone))
	if len(job_ids_to_delete) > 0 {
		yodb.Delete[JobRun](ctx, JobRunId.In(job_ids_to_delete.ToAnys()...))
	}

	// clean up renamed/removed-from-codebase job types
	var job_def_ids_to_delete sl.Of[yodb.I64]
	for _, job_def := range yodb.FindMany[JobDef](ctx, nil, 0, nil /* keep it all-fields due to JobDef.OnAfterLoaded */) {
		if !JobTypeExists(job_def.JobTypeId.String()) {
			job_def_ids_to_delete = append(job_def_ids_to_delete, job_def.Id)
		}
	}
	if len(job_def_ids_to_delete) > 0 {
		yodb.Delete[JobDef](ctx, JobDefId.In(job_def_ids_to_delete.ToAnys()...))
	}
}
