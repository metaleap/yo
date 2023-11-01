package yojobs

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	. "yo/util"
	"yo/util/str"
)

// TimeoutLong is:
//   - the default fallback for `JobDef`s without a custom `Timeouts.TaskRun`.
//   - the default fallback for `JobDef`s without a custom `Timeouts.JobPrepAndFinalize`.
//   - the timeout active during multiple task cancellations. Left-overs are still picked up by a follow-up cancellation-finalizer.
const TimeoutLong = 2 * time.Minute

// All times handled, stored, loaded etc. are relocated into this `timezone`.
// Only set it once no later than `init` time (if at all), and have it be the same every time across restarts (not a candidate for configurability).
var Timezone = time.UTC

type (
	Engine interface {
		// Resume starts the `Engine`, ie. its (from then on) regularly-recurring background watchers.
		// nb. no `Suspend()` yet, currently out of scope.
		Resume()
		// Running returns `true` after `Resume` was called and `false` until then.
		Running() bool
		// CreateJobRun "manually schedules" an off-schedule job at the defified `dueTime`, which if missing (or set in the past) defaults to `timeNow()`.
		CreateJobRun(ctx context.Context, jobDef *JobDef, jobRunId string, dueTime *time.Time, details JobDetails) (jobRun *JobRun, err error)
		// CancelJobRuns marks the defified jobs as `CANCELLING`. The `len(errs)` is always `<= len(jobIDs)`.
		CancelJobRuns(ctx context.Context, jobRunIds ...string) (errs []error)
		// DeleteJobRun clears from storage the defified DONE or CANCELLED `JobRun` and all its `JobTask`s, if any.
		DeleteJobRun(ctx context.Context, jobRunId string) error
		// Stats gathers progress stats of a `JobRun` and its `JobTask`s.
		Stats(ctx context.Context, jobRunId string) (*JobRunStats, error)
		// RetryJobTask retries the defified failed task.
		RetryJobTask(ctx context.Context, jobRunId string, jobTaskId string) (*JobTask, error)

		// OnJobTaskExecuted takes an event handler (only one is kept) to be invoked when a task run has been run (successfully or not) and that run stored
		OnJobTaskExecuted(func(*JobTask, time.Duration))
		// OnJobRunFinalized takes an event handler (only one is kept) that should take care to `nil`-check its `JobRunStats` arg
		OnJobRunFinalized(func(*JobRun, *JobRunStats))
	}
	engine struct {
		running          bool
		storage          storage
		options          Options
		taskCancelers    map[string]func()
		taskCancelersMut sync.Mutex
		eventHandlers    struct {
			onJobTaskExecuted func(*JobTask, time.Duration)
			onJobRunFinalized func(*JobRun, *JobRunStats)
		}
	}
	Options struct {
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
)

func NewEngine(impl Storage, options Options) (Engine, error) {
	err := sanitize[Options](2, 128, strconv.Atoi, map[string]*int{
		"MaxConcurrentOps": &options.MaxConcurrentOps,
		"FetchTasksToRun":  &options.FetchTasksToRun,
	})
	if err == nil {
		err = sanitize[Options](2*time.Second, 22*time.Hour, time.ParseDuration, map[string]*time.Duration{
			"TimeoutShort":                     &options.TimeoutShort,
			"IntervalStartAndFinalizeJobs":     &options.IntervalStartAndFinalizeJobs,
			"IntervalRunTasks":                 &options.IntervalRunTasks,
			"IntervalExpireOrRetryDeadTasks":   &options.IntervalExpireOrRetryDeadTasks,
			"IntervalEnsureJobSchedules":       &options.IntervalEnsureJobSchedules,
			"IntervalDeleteStorageExpiredJobs": &options.IntervalDeleteStorageExpiredJobs,
		})
	}
	if err != nil {
		return nil, err
	}
	return &engine{storage: storage{impl: impl}, options: options, taskCancelers: map[string]func(){}}, nil
}

func (it *engine) Running() bool { return it.running }
func (it *engine) Resume() {
	it.running = true
	doAfter(it.options.IntervalStartAndFinalizeJobs, it.startAndFinalizeJobRuns)
	doAfter(it.options.IntervalRunTasks, it.runJobTasks)
	doAfter(it.options.IntervalExpireOrRetryDeadTasks, it.expireOrRetryDeadJobTasks)
	doAfter(it.options.IntervalDeleteStorageExpiredJobs/10, it.deleteStorageExpiredJobRuns)
	doAfter(Clamp(22*time.Second, 44*time.Second, it.options.IntervalEnsureJobSchedules), it.ensureJobRunSchedules)
}

func (it *engine) CancelJobRuns(ctx context.Context, jobRunIds ...string) (errs []error) {
	if len(jobRunIds) == 0 {
		return
	}
	job_runs, _, _, err := it.storage.listJobRuns(ctx, false, false, ListRequest{PageSize: len(jobRunIds)},
		JobRunFilter{}.WithStates(Running, Pending).WithIds(jobRunIds...))
	if err != nil {
		return []error{err}
	}
	for _, err := range it.cancelJobRuns(ctx, map[CancellationReason][]*JobRun{"": job_runs}) {
		errs = append(errs, err)
	}
	return
}

func (it *engine) cancelJobRuns(ctx context.Context, jobRunsToCancel map[CancellationReason][]*JobRun) (errs map[*JobRun]error) {
	if len(jobRunsToCancel) == 0 {
		return
	}
	log := loggerNew()
	var mut_errs sync.Mutex
	errs = make(map[*JobRun]error, len(jobRunsToCancel)/2)
	for reason, jobRuns := range jobRunsToCancel {
		GoItems(ctx, jobRuns, func(ctx context.Context, jobRun *JobRun) {
			state, version := jobRun.State, jobRun.Version
			jobRun.State, jobRun.Info.CancellationReason = JobRunCancelling, reason
			if it.logLifecycleEvents(nil, jobRun, nil) {
				jobRun.logger(log).Infof("marking %s '%s' job run '%s' as %s", state, jobRun.JobDefId, jobRun.Id, jobRun.State)
			}
			if err := it.storage.saveJobRun(ctx, jobRun); err != nil {
				jobRun.State, jobRun.Version = state, version
				mut_errs.Lock()
				errs[jobRun] = err
				mut_errs.Unlock()
			}
		}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
	}
	return
}

func (it *engine) DeleteJobRun(ctx context.Context, jobRunId string) error {
	job_run, err := it.storage.getJobRun(ctx, false, false, jobRunId)
	if err != nil {
		return err
	}
	if (job_run.State != Done) && (job_run.State != Cancelled) {
		return errors.New(str.Fmt("job run '%s' was expected in a `state` of '%s' or '%s', not '%s'", jobRunId, Done, Cancelled, job_run.State))
	}
	return it.storage.deleteJobRuns(ctx, JobRunFilter{}.WithIds(jobRunId))
}

func (it *engine) CreateJobRun(ctx context.Context, jobDef *JobDef, jobRunId string, dueTime *time.Time, jobDetails JobDetails) (jobRun *JobRun, err error) {
	if now := timeNow(); dueTime == nil {
		dueTime = now
	} else if dueTime = ToPtr(dueTime.In(Timezone)); now.After(*dueTime) {
		dueTime = now
	}
	return it.createJobRun(ctx, jobDef, jobRunId, *dueTime, jobDetails, nil, false)
}

func (it *engine) createJobRun(ctx context.Context, jobDef *JobDef, jobRunId string, dueTime time.Time, jobDetails JobDetails, lastJobRun *JobRun, autoScheduled bool) (jobRun *JobRun, err error) {
	log := loggerNew()
	if jobDef.Disabled {
		return nil, errors.New(str.Fmt("cannot create off-schedule job run for job def '%s' because it is currently disabled", jobDef.Id))
	}
	if (!autoScheduled) && !jobDef.AllowManualJobRuns {
		return nil, errors.New(str.Fmt("cannot create off-schedule job run for job def '%s' because it is configured to not `AllowManualJobRuns`", jobDef.Id))
	}
	if jobRunId == "" {
		jobRunId = newId(jobDef.Id)
	}

	jobRun = &JobRun{
		Id:                       jobRunId,
		JobDefId:                 jobDef.Id,
		JobTypeId:                jobDef.JobTypeId,
		State:                    Pending,
		AutoScheduled:            autoScheduled,
		Version:                  1,
		jobDef:                   jobDef,
		Details:                  jobDetails,
		DueTime:                  dueTime.In(Timezone),
		ScheduledNextAfterJobRun: If(autoScheduled, "_none_", "_manual_") + newId(jobDef.Id),
	}
	if autoScheduled && (lastJobRun != nil) {
		jobRun.ScheduledNextAfterJobRun = lastJobRun.Id
		already_there, err := it.storage.findJobRun(ctx, true, true, JobRunFilter{}.WithScheduledNextAfterJobRun(jobRun.ScheduledNextAfterJobRun))
		if (already_there != nil) || (err != nil) {
			return If((already_there != nil), already_there, jobRun), err
		}
	}
	if it.logLifecycleEvents(nil, jobRun, nil) {
		jobRun.logger(log).Infof("creating %s '%s' job run '%s' scheduled for %s", Pending, jobRun.JobDefId, jobRun.Id, jobRun.DueTime)
	}
	return jobRun, it.storage.insertJobRuns(ctx, jobRun)
}

func (it *engine) RetryJobTask(ctx context.Context, jobRunId string, jobTaskId string) (*JobTask, error) {
	job_task, err := it.storage.getJobTask(ctx, true, true, jobTaskId)
	if err != nil {
		return nil, err
	}
	job_run := job_task.jobRun
	if (job_run == nil) || (job_task.JobRunId != jobRunId) || (job_run.Id != jobRunId) {
		return nil, errors.New(str.Fmt("job run '%s' has no task '%s'", jobRunId, jobTaskId))
	}
	if (job_run.State == JobRunCancelling) || (job_run.State == Cancelled) || (job_run.State == Pending) {
		return nil, errors.New(str.Fmt("'%s' job run '%s' is %s", job_run.JobDefId, jobRunId, job_run.State))
	}
	if (job_task.State != Done) || (len(job_task.Attempts) == 0) || (job_task.Attempts[0].TaskError == nil) {
		return nil, errors.New(str.Fmt("job task '%s' must be in a `state` of %s (currently: %s) with the latest `attempts` (current len: %d) entry having an `error` set", job_task.Id, Done, job_task.State, len(job_task.Attempts)))
	}

	return job_task, it.storage.transacted(ctx, func(ctx context.Context) error {
		if job_run.State != Running {
			log := loggerNew()
			if it.logLifecycleEvents(job_run.jobDef, job_run, job_task) {
				job_run.logger(log).Infof("marking %s '%s' job run '%s' as %s (for manual task retry)", job_run.State, job_run.JobDefId, job_run.Id, Running)
			}
			job_run.State, job_run.FinishTime, job_run.Results, job_run.ResultsStore = Running, nil, nil, nil
			if err := it.storage.saveJobRun(ctx, job_run); err != nil {
				return err
			}
		}
		return it.runTask(ctx, job_task)
	})
}

func (it *engine) Stats(ctx context.Context, jobRunId string) (*JobRunStats, error) {
	job_run, err := it.storage.getJobRun(ctx, false, false, jobRunId)
	if err != nil {
		return nil, err
	}
	return it.stats(ctx, job_run)
}

func (it *engine) stats(ctx context.Context, jobRun *JobRun) (*JobRunStats, error) {
	var err error
	stats := JobRunStats{TasksByState: make(map[RunState]int64, 4)}
	for _, state := range []RunState{Pending, Running, Done, Cancelled} {
		if stats.TasksByState[state], err = it.storage.countJobTasks(ctx, 0,
			JobTaskFilter{}.WithJobRuns(jobRun.Id).WithStates(state),
		); err != nil {
			return nil, err
		}
		stats.TasksTotal += stats.TasksByState[state]
	}
	if stats.TasksFailed, err = it.storage.countJobTasks(ctx, 0,
		JobTaskFilter{}.WithJobRuns(jobRun.Id).WithStates(Done).WithFailed(),
	); err != nil {
		return nil, err
	}
	stats.TasksSucceeded = stats.TasksByState[Done] - stats.TasksFailed

	if (jobRun.StartTime != nil) && (jobRun.FinishTime != nil) {
		stats.DurationTotalMins = ToPtr(jobRun.FinishTime.Sub(*jobRun.StartTime).Minutes())
	}
	stats.DurationPrepSecs, stats.DurationFinalizeSecs = jobRun.Info.DurationPrepSecs, jobRun.Info.DurationFinalizeSecs
	return &stats, err
}

func (it *engine) OnJobTaskExecuted(eventHandler func(*JobTask, time.Duration)) {
	it.eventHandlers.onJobTaskExecuted = eventHandler
}
func (it *engine) OnJobRunFinalized(eventHandler func(*JobRun, *JobRunStats)) {
	it.eventHandlers.onJobRunFinalized = eventHandler
}
