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
		// JobRunStats gathers progress stats of a `Job` and its `Task`s.
		JobRunStats(ctx context.Context, jobRun Resource) (*JobRunStats, error)
		// RetryJobTask retries the defified failed task.
		RetryJobTask(ctx context.Context, jobRunId string, jobTaskId string) (*Task, error)

		OnJobTaskExecuted(func(*Task, time.Duration))
		OnJobRunExecuted(func(*Job, *JobRunStats))
	}
	engine struct {
		running          bool
		backend          backend
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

		// IntervalStartAndFinalizeJobRuns should be under 0.5 minutes.
		IntervalStartAndFinalizeJobRuns time.Duration `default:"22s"`
		// IntervalRunTasks should be under 0.5 minutes.
		IntervalRunTasks time.Duration `default:"11s"`
		// IntervalExpireOrRetryDeadTasks is advised every couple of minutes (under 5). It ensures (in storage) retry-or-done-with-error of tasks whose last runner died between their completion and updating their Result and RunState in storage accordingly.
		IntervalExpireOrRetryDeadTasks time.Duration `default:"3m"`
		// IntervalEnsureJobSchedules is advised every couple of minutes (under 5). It is only there to catch up scheduling-wise with new or changed `JobDef`s; otherwise a finalized `Job` gets its next occurrence scheduled right at finalization.
		IntervalEnsureJobSchedules time.Duration `default:"2m"`
		// IntervalDeleteStorageExpiredJobRuns can be on the order of hours: job storage-expiry is set in number-of-days.
		IntervalDeleteStorageExpiredJobRuns time.Duration `default:"5h"`

		// TimeoutShort is the usual timeout for most timeoutable calls (ie. brief DB queries and simple non-batch, non-transaction updates).
		// It should be well under 1min, and is not applicable for the cases described for `const TimeoutLong`.
		TimeoutShort time.Duration `default:"22s"`
		// MaxConcurrentOps limits parallelism when processing multiple `Resource`s concurrently.
		MaxConcurrentOps int `default:"6"`
		// FetchTasksToRun denotes the maximum number of tasks-to-run-now to fetch, approx. every `IntervalRunTasks`.
		FetchTasksToRun int `default:"3"`
	}
)

func NewEngine(impl Backend, options Options) (Engine, error) {
	err := sanitize[Options](2, 128, strconv.Atoi, map[string]*int{
		"MaxConcurrentOps": &options.MaxConcurrentOps,
		"FetchTasksToRun":  &options.FetchTasksToRun,
	})
	if err == nil {
		err = sanitize[Options](2*time.Second, 22*time.Hour, time.ParseDuration, map[string]*time.Duration{
			"TimeoutShort":                     &options.TimeoutShort,
			"IntervalStartAndFinalizeJobs":     &options.IntervalStartAndFinalizeJobRuns,
			"IntervalRunTasks":                 &options.IntervalRunTasks,
			"IntervalExpireOrRetryDeadTasks":   &options.IntervalExpireOrRetryDeadTasks,
			"IntervalEnsureJobSchedules":       &options.IntervalEnsureJobSchedules,
			"IntervalDeleteStorageExpiredJobs": &options.IntervalDeleteStorageExpiredJobRuns,
		})
	}
	if err != nil {
		return nil, err
	}
	return &engine{backend: backend{impl: impl}, options: options, taskCancelers: map[string]func(){}}, nil
}

func (it *engine) Running() bool { return it.running }
func (it *engine) Resume() {
	it.running = true
	doAfter(it.options.IntervalStartAndFinalizeJobRuns, it.startAndFinalizeJobs)
	doAfter(it.options.IntervalRunTasks, it.runTasks)
	doAfter(it.options.IntervalExpireOrRetryDeadTasks, it.expireOrRetryDeadTasks)
	doAfter(it.options.IntervalDeleteStorageExpiredJobRuns/10, it.deleteStorageExpiredJobs)
	doAfter(Clamp(22*time.Second, 44*time.Second, it.options.IntervalEnsureJobSchedules), it.ensureJobSchedules)
}

func (it *engine) CancelJobRuns(ctx context.Context, jobIDs ...string) (errs []error) {
	jobs, _, _, err := it.backend.listJobRuns(ctx, false, false, ListRequest{PageSize: len(jobIDs)},
		JobFilter{}.WithStates(Running, Pending).WithIDs(jobIDs...))
	if err != nil {
		return []error{err}
	}
	for _, err := range it.cancelJobs(ctx, map[CancellationReason][]*Job{"": jobs}) {
		errs = append(errs, err)
	}
	return
}

func (it *engine) cancelJobs(ctx context.Context, jobs map[CancellationReason][]*Job) (errs map[*Job]error) {
	log := loggerNew()
	var mut sync.Mutex
	errs = make(map[*Job]error, len(jobs)/2)
	for reason, jobs := range jobs {
		GoItems(ctx, jobs, func(ctx context.Context, job *Job) {
			state, version := job.State, job.ResourceVersion
			job.State, job.Info.CancellationReason = Cancelling, reason
			if it.logLifecycleEvents(false, nil, job, nil) {
				job.logger(log).Infof("marking %s '%s' job '%s' as %s", state, job.Def, job.Id, job.State)
			}
			if err := it.backend.saveJobRun(ctx, job); err != nil {
				job.State, job.ResourceVersion = state, version
				mut.Lock()
				errs[job] = err
				mut.Unlock()
			}
		}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
	}
	return
}

func (it *engine) DeleteJobRun(ctx context.Context, jobRef Resource) error {
	job, err := it.backend.getJobRun(ctx, false, false, jobRef.Id)
	if err != nil {
		return err
	}
	if job.State != Done && job.State != Cancelled {
		return errors.New(str.Fmt("job '%s' was expected in a `state` of '%s' or '%s', not '%s'", jobRef.Id, Done, Cancelled, job.State))
	}
	return it.backend.deleteJobRuns(ctx, JobFilter{}.WithIDs(jobRef.Id))
}

func (it *engine) CreateJobRun(ctx context.Context, jobDef *JobDef, jobID string, dueTime *time.Time, details JobDetails) (job *Job, err error) {
	if now := timeNow(); dueTime == nil {
		dueTime = now
	} else if dueTime = ToPtr(dueTime.In(Timezone)); now.After(*dueTime) {
		dueTime = now
	}
	return it.createJob(ctx, jobDef, jobID, *dueTime, details, nil, false)
}

func (it *engine) createJob(ctx context.Context, jobDef *JobDef, jobID string, dueTime time.Time, details JobDetails, last *Job, autoScheduled bool) (job *Job, err error) {
	log := loggerNew()
	if jobDef.Disabled {
		return nil, errors.New(str.Fmt("cannot create off-schedule Job for job def '%s' because it is currently disabled", jobDef.Id))
	}
	if !autoScheduled && !jobDef.AllowManualJobs {
		return nil, errors.New(str.Fmt("cannot create off-schedule Job for job def '%s' because it is configured to not `allowManualJobs`", jobDef.Id))
	}
	if jobID == "" {
		jobID = newId(jobDef.Id)
	}

	job = &Job{
		Resource:              Resource{jobID},
		Def:                   jobDef.Id,
		HandlerID:             jobDef.HandlerID,
		State:                 Pending,
		AutoScheduled:         autoScheduled,
		ResourceVersion:       1,
		def:                   jobDef,
		Details:               details,
		DueTime:               dueTime.In(Timezone),
		ScheduledNextAfterJob: If(autoScheduled, "_none_", "_manual_") + newId(jobDef.Id),
	}
	if autoScheduled && last != nil {
		job.ScheduledNextAfterJob = last.Id
		alreadyThere, err := it.backend.findJobRun(ctx, true, true, JobFilter{}.WithScheduledNextAfterJob(job.ScheduledNextAfterJob))
		if alreadyThere != nil || err != nil {
			return If(alreadyThere != nil, alreadyThere, job), err
		}
	}
	if it.logLifecycleEvents(false, nil, job, nil) {
		job.logger(log).Infof("creating %s '%s' job '%s' scheduled for %s", Pending, job.Def, job.Id, job.DueTime)
	}
	return job, it.backend.insertJobRuns(ctx, job)
}

func (it *engine) RetryJobTask(ctx context.Context, jobID string, taskID string) (*Task, error) {
	task, err := it.backend.getJobTask(ctx, true, true, taskID)
	if err != nil {
		return nil, err
	}
	job := task.job
	if job == nil || task.Job != jobID || job.Id != jobID {
		return nil, errors.New(str.Fmt("job '%s' has no task '%s'", jobID, taskID))
	}
	if job.State == Cancelling || job.State == Cancelled || job.State == Pending {
		return nil, errors.New(str.Fmt("'%s' job '%s' is %s", job.Def, jobID, job.State))
	}
	if task.State != Done || len(task.Attempts) == 0 || task.Attempts[0].TaskError == nil {
		return nil, errors.New(str.Fmt("job task '%s' must be in a `state` of %s (currently: %s) with the latest `attempts` (current len: %d) entry having an `error` set", task.Id, Done, task.State, len(task.Attempts)))
	}

	return task, it.backend.transacted(ctx, func(ctx context.Context) error {
		if job.State != Running {
			log := loggerNew()
			if it.logLifecycleEvents(true, job.def, job, task) {
				job.logger(log).Infof("marking %s '%s' job '%s' as %s (for manual task retry)", job.State, job.Def, job.Id, Running)
			}
			job.State, job.FinishTime, job.Results, job.ResultsStore = Running, nil, nil, nil
			if err := it.backend.saveJobRun(ctx, job); err != nil {
				return err
			}
		}
		return it.runTask(ctx, task)
	})
}

func (it *engine) JobRunStats(ctx context.Context, jobRef Resource) (*JobRunStats, error) {
	job, err := it.backend.getJobRun(ctx, false, false, jobRef.Id)
	if err != nil {
		return nil, err
	}

	ret := JobRunStats{TasksByState: make(map[RunState]int64, 4)}
	for _, state := range []RunState{Pending, Running, Done, Cancelled} {
		ret.TasksByState[state], err = it.backend.countJobTasks(ctx, 0,
			TaskFilter{}.WithJobs(job.Id).WithStates(state))
		if ret.TasksTotal += ret.TasksByState[state]; err != nil {
			return nil, err
		}
	}
	if ret.TasksFailed, err = it.backend.countJobTasks(ctx, 0,
		TaskFilter{}.WithJobs(job.Id).WithStates(Done).WithFailed()); err != nil {
		return nil, err
	}
	ret.TasksSucceeded = ret.TasksByState[Done] - ret.TasksFailed

	if job.StartTime != nil && job.FinishTime != nil {
		ret.DurationTotalMins = ToPtr(job.FinishTime.Sub(*job.StartTime).Minutes())
	}
	ret.DurationPrepMins, ret.DurationFinalizeMins = job.Info.DurationPrepInMinutes, job.Info.DurationFinalizeInMinutes
	return &ret, err
}

func (it *engine) OnJobTaskExecuted(eventHandler func(*Task, time.Duration)) {
	it.eventHandlers.onJobTaskExecuted = eventHandler
}
func (it *engine) OnJobRunExecuted(eventHandler func(*Job, *JobRunStats)) {
	it.eventHandlers.onJobRunExecuted = eventHandler
}
