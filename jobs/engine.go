package jobs

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
		CreateJobRun(ctx context.Context, jobDef *JobDef, jobRunId string, dueTime *time.Time, details JobDetails) (jobRun *Job, err error)
		// CancelJobRuns marks the defified jobs as `CANCELLING`. The `len(errs)` is always `<= len(jobIDs)`.
		CancelJobRuns(ctx context.Context, jobRunIds ...string) (errs []error)
		// DeleteJobRun clears from storage the defified DONE or CANCELLED `Job` and all its `Task`s, if any.
		DeleteJobRun(ctx context.Context, jobRun Resource) error
		// Stats gathers progress stats of a `Job` and its `Task`s.
		Stats(ctx context.Context, jobRun Resource) (*JobRunStats, error)
		// RetryJobTask retries the defified failed task.
		RetryJobTask(ctx context.Context, jobRunId string, jobTaskId string) (*Task, error)

		OnJobTaskExecuted(func(*Task, time.Duration))
		OnJobRunExecuted(func(*Job, *JobRunStats))
	}
	engine struct {
		running          bool
		backend          store
		options          Options
		taskCancelers    map[string]func()
		taskCancelersMut sync.Mutex
		eventHandlers    struct {
			onJobTaskExecuted func(*Task, time.Duration)
			onJobRunExecuted  func(*Job, *JobRunStats)
		}
	}
	Options struct {
		// LogJobLifecycleEvents enables the info-level logging of every attempted RunState transition of all `Job`s.
		LogJobLifecycleEvents bool
		// LogTaskLifecycleEvents enables the info-level logging of every attempted RunState transition of all `Task`s.
		LogTaskLifecycleEvents bool

		// these "intervals" aren't "every n, do" but more like "wait n to go again _after_ done"

		// IntervalStartAndFinalizeJobs should be under 0.5 minutes.
		IntervalStartAndFinalizeJobs time.Duration `default:"22s"`
		// IntervalRunTasks should be under 0.5 minutes.
		IntervalRunTasks time.Duration `default:"11s"`
		// IntervalExpireOrRetryDeadTasks is advised every couple of minutes (under 5). It ensures (in storage) retry-or-done-with-error of tasks whose last runner died between their completion and updating their Result and RunState in storage accordingly.
		IntervalExpireOrRetryDeadTasks time.Duration `default:"3m"`
		// IntervalEnsureJobSchedules is advised every couple of minutes (under 5). It is only there to catch up scheduling-wise with new or changed `JobDef`s; otherwise a finalized `Job` gets its next occurrence scheduled right at finalization.
		IntervalEnsureJobSchedules time.Duration `default:"2m"`
		// IntervalDeleteStorageExpiredJobs can be on the order of hours: job storage-expiry is set in number-of-days.
		IntervalDeleteStorageExpiredJobs time.Duration `default:"5h"`

		// TimeoutShort is the usual timeout for most timeoutable calls (ie. brief DB queries and simple non-batch, non-transaction updates).
		// It should be well under 1min, and is not applicable for the cases described for `const TimeoutLong`.
		TimeoutShort time.Duration `default:"22s"`
		// MaxConcurrentOps limits parallelism when processing multiple `Resource`s concurrently.
		MaxConcurrentOps int `default:"6"`
		// FetchTasksToRun denotes the maximum number of tasks-to-run-now to fetch, approx. every `IntervalRunTasks`.
		FetchTasksToRun int `default:"3"`
	}
)

func NewEngine(impl Store, options Options) (Engine, error) {
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
	return &engine{backend: store{impl: impl}, options: options, taskCancelers: map[string]func(){}}, nil
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
	job_runs, _, _, err := it.backend.listJobRuns(ctx, false, false, ListRequest{PageSize: len(jobRunIds)},
		JobRunFilter{}.WithStates(Running, Pending).WithIds(jobRunIds...))
	if err != nil {
		return []error{err}
	}
	for _, err := range it.cancelJobRuns(ctx, map[CancellationReason][]*Job{"": job_runs}) {
		errs = append(errs, err)
	}
	return
}

func (it *engine) cancelJobRuns(ctx context.Context, jobRuns map[CancellationReason][]*Job) (errs map[*Job]error) {
	log := loggerNew()
	var mut_errs sync.Mutex
	errs = make(map[*Job]error, len(jobRuns)/2)
	for reason, jobs := range jobRuns {
		GoItems(ctx, jobs, func(ctx context.Context, jobRun *Job) {
			state, version := jobRun.State, jobRun.ResourceVersion
			jobRun.State, jobRun.Info.CancellationReason = Cancelling, reason
			if it.logLifecycleEvents(false, nil, jobRun, nil) {
				jobRun.logger(log).Infof("marking %s '%s' job run '%s' as %s", state, jobRun.Def, jobRun.Id, jobRun.State)
			}
			if err := it.backend.saveJobRun(ctx, jobRun); err != nil {
				jobRun.State, jobRun.ResourceVersion = state, version
				mut_errs.Lock()
				errs[jobRun] = err
				mut_errs.Unlock()
			}
		}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
	}
	return
}

func (it *engine) DeleteJobRun(ctx context.Context, jobRunRef Resource) error {
	job_run, err := it.backend.getJobRun(ctx, false, false, jobRunRef.Id)
	if err != nil {
		return err
	}
	if (job_run.State != Done) && (job_run.State != Cancelled) {
		return errors.New(str.Fmt("job run '%s' was expected in a `state` of '%s' or '%s', not '%s'", jobRunRef.Id, Done, Cancelled, job_run.State))
	}
	return it.backend.deleteJobRuns(ctx, JobRunFilter{}.WithIds(jobRunRef.Id))
}

func (it *engine) CreateJobRun(ctx context.Context, jobDef *JobDef, jobRunId string, dueTime *time.Time, jobDetails JobDetails) (jobRun *Job, err error) {
	if now := timeNow(); dueTime == nil {
		dueTime = now
	} else if dueTime = ToPtr(dueTime.In(Timezone)); now.After(*dueTime) {
		dueTime = now
	}
	return it.createJobRun(ctx, jobDef, jobRunId, *dueTime, jobDetails, nil, false)
}

func (it *engine) createJobRun(ctx context.Context, jobDef *JobDef, jobRunId string, dueTime time.Time, jobDetails JobDetails, lastJobRun *Job, autoScheduled bool) (jobRun *Job, err error) {
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

	jobRun = &Job{
		Resource:              Resource{jobRunId},
		Def:                   jobDef.Id,
		HandlerID:             jobDef.HandlerID,
		State:                 Pending,
		AutoScheduled:         autoScheduled,
		ResourceVersion:       1,
		def:                   jobDef,
		Details:               jobDetails,
		DueTime:               dueTime.In(Timezone),
		ScheduledNextAfterJob: If(autoScheduled, "_none_", "_manual_") + newId(jobDef.Id),
	}
	if autoScheduled && (lastJobRun != nil) {
		jobRun.ScheduledNextAfterJob = lastJobRun.Id
		already_there, err := it.backend.findJobRun(ctx, true, true, JobRunFilter{}.WithScheduledNextAfterJobRun(jobRun.ScheduledNextAfterJob))
		if (already_there != nil) || (err != nil) {
			return If((already_there != nil), already_there, jobRun), err
		}
	}
	if it.logLifecycleEvents(false, nil, jobRun, nil) {
		jobRun.logger(log).Infof("creating %s '%s' job run '%s' scheduled for %s", Pending, jobRun.Def, jobRun.Id, jobRun.DueTime)
	}
	return jobRun, it.backend.insertJobRuns(ctx, jobRun)
}

func (it *engine) RetryJobTask(ctx context.Context, jobRunId string, jobTaskId string) (*Task, error) {
	job_task, err := it.backend.getJobTask(ctx, true, true, jobTaskId)
	if err != nil {
		return nil, err
	}
	job_run := job_task.job
	if (job_run == nil) || (job_task.Job != jobRunId) || (job_run.Id != jobRunId) {
		return nil, errors.New(str.Fmt("job run '%s' has no task '%s'", jobRunId, jobTaskId))
	}
	if (job_run.State == Cancelling) || (job_run.State == Cancelled) || (job_run.State == Pending) {
		return nil, errors.New(str.Fmt("'%s' job run '%s' is %s", job_run.Def, jobRunId, job_run.State))
	}
	if (job_task.State != Done) || (len(job_task.Attempts) == 0) || (job_task.Attempts[0].TaskError == nil) {
		return nil, errors.New(str.Fmt("job task '%s' must be in a `state` of %s (currently: %s) with the latest `attempts` (current len: %d) entry having an `error` set", job_task.Id, Done, job_task.State, len(job_task.Attempts)))
	}

	return job_task, it.backend.transacted(ctx, func(ctx context.Context) error {
		if job_run.State != Running {
			log := loggerNew()
			if it.logLifecycleEvents(true, job_run.def, job_run, job_task) {
				job_run.logger(log).Infof("marking %s '%s' job run '%s' as %s (for manual task retry)", job_run.State, job_run.Def, job_run.Id, Running)
			}
			job_run.State, job_run.FinishTime, job_run.Results, job_run.ResultsStore = Running, nil, nil, nil
			if err := it.backend.saveJobRun(ctx, job_run); err != nil {
				return err
			}
		}
		return it.runTask(ctx, job_task)
	})
}

func (it *engine) Stats(ctx context.Context, jobRunRef Resource) (*JobRunStats, error) {
	job_run, err := it.backend.getJobRun(ctx, false, false, jobRunRef.Id)
	if err != nil {
		return nil, err
	}

	stats := JobRunStats{TasksByState: make(map[RunState]int64, 4)}
	for _, state := range []RunState{Pending, Running, Done, Cancelled} {
		if stats.TasksByState[state], err = it.backend.countJobTasks(ctx, 0,
			JobTaskFilter{}.WithJobRuns(job_run.Id).WithStates(state),
		); err != nil {
			return nil, err
		}
		stats.TasksTotal += stats.TasksByState[state]
	}
	if stats.TasksFailed, err = it.backend.countJobTasks(ctx, 0,
		JobTaskFilter{}.WithJobRuns(job_run.Id).WithStates(Done).WithFailed(),
	); err != nil {
		return nil, err
	}
	stats.TasksSucceeded = stats.TasksByState[Done] - stats.TasksFailed

	if (job_run.StartTime != nil) && (job_run.FinishTime != nil) {
		stats.DurationTotalMins = ToPtr(job_run.FinishTime.Sub(*job_run.StartTime).Minutes())
	}
	stats.DurationPrepMins, stats.DurationFinalizeMins = job_run.Info.DurationPrepInMinutes, job_run.Info.DurationFinalizeInMinutes
	return &stats, err
}

func (it *engine) OnJobTaskExecuted(eventHandler func(*Task, time.Duration)) {
	it.eventHandlers.onJobTaskExecuted = eventHandler
}
func (it *engine) OnJobRunExecuted(eventHandler func(*Job, *JobRunStats)) {
	it.eventHandlers.onJobRunExecuted = eventHandler
}
