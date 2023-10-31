package jobs

import (
	"context"
	"strconv"
	"sync"
	"time"

	"jobs/pkg/utils"

	errors "go.enpowerx.io/errors"
)

// TimeoutLong is:
//   - the default fallback for `JobSpec`s without a custom `Timeouts.TaskRun`.
//   - the default fallback for `JobSpec`s without a custom `Timeouts.JobPrepAndFinalize`.
//   - the timeout active during multiple task cancellations. Left-overs are still picked up by a follow-up cancellation-finalizer.
const TimeoutLong = 2 * time.Minute

// All times handled, stored, loaded etc. are relocated into this `timezone`.
// Only set it once no later than `init` time (if at all), and have it be the same every time across restarts (not a candidate for configurability).
var Timezone = time.UTC

type (
	Engine interface {
		// Resume starts the `Engine`, ie. its (from then on) regularly-recurring background watchers.
		// TODO: no `Suspend()` yet, currently out of scope.
		Resume()
		// Running returns `true` after `Resume` was called and `false` until then.
		Running() bool
		// CreateJob "manually schedules" an off-schedule job at the specified `dueTime`, which if missing (or set in the past) defaults to `timeNow()`.
		CreateJob(ctx context.Context, jobSpec *JobSpec, jobID string, dueTime *time.Time, details JobDetails) (job *Job, err error)
		// CancelJobs marks the specified jobs as `CANCELLING`. The `len(errs)` is always `<= len(jobIDs)`.
		CancelJobs(ctx context.Context, tenant string, jobIDs ...string) (errs []error)
		// DeleteJob clears from storage the specified DONE or CANCELLED `Job` and all its `Task`s, if any.
		DeleteJob(ctx context.Context, job Resource) error
		// JobStats gathers progress stats of a `Job` and its `Task`s.
		JobStats(ctx context.Context, job Resource) (*JobStats, error)
		// RetryTask retries the specified failed task.
		RetryTask(ctx context.Context, tenant string, jobID string, taskID string) (*Task, error)

		OnTaskExecuted(func(*Task, time.Duration))
		OnJobExecuted(func(*Job, *JobStats))
	}
	engine struct {
		running          bool
		backend          backend
		options          Options
		taskCancelers    map[string]func()
		taskCancelersMut sync.Mutex
		eventHandlers    struct {
			OnTaskExecuted func(*Task, time.Duration)
			OnJobExecuted  func(*Job, *JobStats)
		}
	}
	Options struct {
		// LogJobLifecycleEvents enables the info-level logging of every attempted RunState transition of all `Job`s.
		LogJobLifecycleEvents bool `yaml:"logJobLifecycleEvents"`
		// LogTaskLifecycleEvents enables the info-level logging of every attempted RunState transition of all `Task`s.
		LogTaskLifecycleEvents bool `yaml:"logTaskLifecycleEvents"`

		// these "intervals" aren't "every n, do" but more like "wait n to go again _after_ done"

		// IntervalStartAndFinalizeJobs should be under 0.5 minutes.
		IntervalStartAndFinalizeJobs time.Duration `yaml:"intervalStartAndFinalizeJobs" default:"22s"`
		// IntervalRunTasks should be under 0.5 minutes.
		IntervalRunTasks time.Duration `yaml:"intervalRunTasks" default:"11s"`
		// IntervalExpireOrRetryDeadTasks is advised every couple of minutes (under 5). It ensures (in storage) retry-or-done-with-error of tasks whose last runner died between their completion and updating their Result and RunState in storage accordingly.
		IntervalExpireOrRetryDeadTasks time.Duration `yaml:"intervalExpireOrRetryDeadTasks" default:"3m"`
		// IntervalEnsureJobSchedules is advised every couple of minutes (under 5). It is only there to catch up scheduling-wise with new or changed `JobSpec`s; otherwise a finalized `Job` gets its next occurrence scheduled right at finalization.
		IntervalEnsureJobSchedules time.Duration `yaml:"intervalEnsureJobSchedules" default:"2m"`
		// IntervalDeleteStorageExpiredJobs can be on the order of hours: job storage-expiry is set in number-of-days.
		IntervalDeleteStorageExpiredJobs time.Duration `yaml:"intervalDeleteStorageExpiredJobs" default:"5h"`

		// TimeoutShort is the usual timeout for most timeoutable calls (ie. brief DB queries and simple non-batch, non-transaction updates).
		// It should be well under 1min, and is not applicable for the cases described for `const TimeoutLong`.
		TimeoutShort time.Duration `yaml:"defaultTimeout" default:"22s"`
		// MaxConcurrentOps limits parallelism when processing multiple `Resource`s concurrently.
		MaxConcurrentOps int `yaml:"maxConcurrentOps" default:"6"`
		// FetchTasksToRunPerTenant denotes the maximum number of tasks-to-run-now to fetch, per tenant, approx. every `IntervalRunTasks`.
		FetchTasksToRunPerTenant int `yaml:"fetchTasksToRunPerTenant" default:"3"`
	}
)

func NewEngine(impl Backend, options Options) (Engine, error) {
	err := sanitize[Options](2, 128, strconv.Atoi, map[string]*int{
		"MaxConcurrentOps":         &options.MaxConcurrentOps,
		"FetchTasksToRunPerTenant": &options.FetchTasksToRunPerTenant,
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
	return &engine{backend: backend{impl: impl}, options: options, taskCancelers: map[string]func(){}}, nil
}

func (it *engine) Running() bool { return it.running }
func (it *engine) Resume() {
	it.running = true
	doAfter(it.options.IntervalStartAndFinalizeJobs, it.startAndFinalizeJobs)
	doAfter(it.options.IntervalRunTasks, it.runTasks)
	doAfter(it.options.IntervalExpireOrRetryDeadTasks, it.expireOrRetryDeadTasks)
	doAfter(it.options.IntervalDeleteStorageExpiredJobs/10, it.deleteStorageExpiredJobs)
	doAfter(clamp(22*time.Second, 44*time.Second, it.options.IntervalEnsureJobSchedules), it.ensureJobSchedules)
}

func (it *engine) CancelJobs(ctx context.Context, tenant string, jobIDs ...string) (errs []error) {
	jobs, _, _, err := it.backend.listJobs(ctx, false, false, tenant, ListRequest{PageSize: len(jobIDs)},
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
	log := loggerFor(ctx)
	var mut sync.Mutex
	errs = make(map[*Job]error, len(jobs)/2)
	for reason, jobs := range jobs {
		concurrentlyDo(ctx, jobs, func(ctx context.Context, job *Job) {
			state, version := job.State, job.ResourceVersion
			job.State, job.Info.CancellationReason = Cancelling, reason
			if it.logLifecycleEvents(false, nil, job, nil) {
				job.logger(log).Infof("marking %s '%s' job '%s' as %s", state, job.Spec, job.ID, job.State)
			}
			if err := it.backend.saveJob(ctx, job); err != nil {
				job.State, job.ResourceVersion = state, version
				mut.Lock()
				errs[job] = err
				mut.Unlock()
			}
		}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
	}
	return
}

func (it *engine) DeleteJob(ctx context.Context, jobRef Resource) error {
	job, err := it.backend.getJob(ctx, false, false, jobRef.Tenant, jobRef.ID)
	if err != nil {
		return err
	}
	if job.State != Done && job.State != Cancelled {
		return errors.FailedPrecondition.New("job '%s' was expected in a `state` of '%s' or '%s', not '%s'", jobRef.ID, Done, Cancelled, job.State)
	}
	return it.backend.deleteJobs(ctx, jobRef.Tenant, JobFilter{}.WithIDs(jobRef.ID))
}

func (it *engine) CreateJob(ctx context.Context, jobSpec *JobSpec, jobID string, dueTime *time.Time, details JobDetails) (job *Job, err error) {
	if now := timeNow(); dueTime == nil {
		dueTime = now
	} else if dueTime = utils.Ptr(dueTime.In(Timezone)); now.After(*dueTime) {
		dueTime = now
	}
	return it.createJob(ctx, jobSpec, jobID, *dueTime, details, nil, false)
}

func (it *engine) createJob(ctx context.Context, jobSpec *JobSpec, jobID string, dueTime time.Time, details JobDetails, last *Job, autoScheduled bool) (job *Job, err error) {
	log := loggerFor(ctx)
	if jobSpec.Disabled {
		return nil, errors.FailedPrecondition.New("cannot create off-schedule Job for job spec '%s' because it is currently disabled", jobSpec.ID)
	}
	if !autoScheduled && !jobSpec.AllowManualJobs {
		return nil, errors.FailedPrecondition.New("cannot create off-schedule Job for job spec '%s' because it is configured to not `allowManualJobs`", jobSpec.ID)
	}
	if jobID == "" {
		if autoScheduled {
			jobID = jobSpec.ID + "_" + strconv.FormatInt(dueTime.UnixNano(), 36) // no ulid/guid etc to avoid the ca. 1x/century accidental duplicate scheduling by multiple concurrent pods
		} else if jobID, err = newULID(); err != nil {
			return nil, err
		}
	}

	dummyID, _ := newULID()
	job = &Job{
		Resource:        Resource{jobSpec.Tenant, jobID},
		Spec:            jobSpec.ID,
		HandlerID:       jobSpec.HandlerID,
		State:           Pending,
		AutoScheduled:   autoScheduled,
		ResourceVersion: 1,
		spec:            jobSpec,
		Details:         details,
		DueTime:         dueTime.In(Timezone),
		ScheduledNextAfterJob: utils.If(autoScheduled, "_none_", "_manual_") +
			utils.If(dummyID != "", dummyID, strconv.FormatInt(time.Now().UnixNano(), 36)),
	}
	if autoScheduled && last != nil {
		job.ScheduledNextAfterJob = last.ID
		alreadyThere, err := it.backend.findJob(ctx, true, true, jobSpec.Tenant, JobFilter{}.WithScheduledNextAfterJob(job.ScheduledNextAfterJob))
		if alreadyThere != nil || err != nil {
			return utils.If(alreadyThere != nil, alreadyThere, job), err
		}
	}
	if it.logLifecycleEvents(false, nil, job, nil) {
		job.logger(log).Infof("creating %s '%s' job '%s' scheduled for %s", Pending, job.Spec, job.ID, job.DueTime)
	}
	return job, it.backend.insertJobs(ctx, job)
}

func (it *engine) RetryTask(ctx context.Context, tenant string, jobID string, taskID string) (*Task, error) {
	task, err := it.backend.getTask(ctx, true, true, tenant, taskID)
	if err != nil {
		return nil, err
	}
	job := task.job
	if job == nil || task.Job != jobID || job.ID != jobID {
		return nil, errors.NotFound.New("job '%s' has no task '%s'", jobID, taskID)
	}
	if job.State == Cancelling || job.State == Cancelled || job.State == Pending {
		return nil, errors.FailedPrecondition.New("'%s' job '%s' is %s", job.Spec, jobID, job.State)
	}
	if task.State != Done || len(task.Attempts) == 0 || task.Attempts[0].TaskError == nil {
		return nil, errors.FailedPrecondition.New("job task '%s' must be in a `state` of %s (currently: %s) with the latest `attempts` (current len: %d) entry having an `error` set", task.ID, Done, task.State, len(task.Attempts))
	}

	return task, it.backend.transacted(ctx, func(ctx context.Context) error {
		if job.State != Running {
			log := loggerFor(ctx)
			if it.logLifecycleEvents(true, job.spec, job, task) {
				job.logger(log).Infof("marking %s '%s' job '%s' as %s (for manual task retry)", job.State, job.Spec, job.ID, Running)
			}
			job.State, job.FinishTime, job.Results, job.ResultsStore = Running, nil, nil, nil
			if err := it.backend.saveJob(ctx, job); err != nil {
				return err
			}
		}
		return it.runTask(ctx, task)
	})
}

func (it *engine) JobStats(ctx context.Context, jobRef Resource) (*JobStats, error) {
	job, err := it.backend.getJob(ctx, false, false, jobRef.Tenant, jobRef.ID)
	if err != nil {
		return nil, err
	}

	ret := JobStats{TasksByState: make(map[RunState]int64, 4)}
	for _, state := range []RunState{Pending, Running, Done, Cancelled} {
		ret.TasksByState[state], err = it.backend.countTasks(ctx, job.Tenant, 0,
			TaskFilter{}.WithJobs(job.ID).WithStates(state))
		if ret.TasksTotal += ret.TasksByState[state]; err != nil {
			return nil, err
		}
	}
	if ret.TasksFailed, err = it.backend.countTasks(ctx, job.Tenant, 0,
		TaskFilter{}.WithJobs(job.ID).WithStates(Done).WithFailed()); err != nil {
		return nil, err
	}
	ret.TasksSucceeded = ret.TasksByState[Done] - ret.TasksFailed

	if job.StartTime != nil && job.FinishTime != nil {
		ret.DurationTotalMins = utils.Ptr(job.FinishTime.Sub(*job.StartTime).Minutes())
	}
	ret.DurationPrepMins, ret.DurationFinalizeMins = job.Info.DurationPrepInMinutes, job.Info.DurationFinalizeInMinutes
	return &ret, err
}

func (it *engine) tenants() (tenants []string) {
	utils.WithTimeoutDo(ctxNone, it.options.TimeoutShort, func(ctx context.Context) {
		allTenants, err := it.backend.tenants(ctx)
		tenants, _ = allTenants, it.logErr(loggerFor(ctx), err, nil)
	})
	return
}

func (it *engine) OnTaskExecuted(eventHandler func(*Task, time.Duration)) {
	it.eventHandlers.OnTaskExecuted = eventHandler
}
func (it *engine) OnJobExecuted(eventHandler func(*Job, *JobStats)) {
	it.eventHandlers.OnJobExecuted = eventHandler
}
