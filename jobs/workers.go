package jobs

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	. "yo/util"
	"yo/util/sl"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	doAfter  = time.AfterFunc
	ctxNone  = context.Background()
	noPaging = ListRequest{PageSize: math.MaxInt32}
)

func (it *engine) startAndFinalizeJobs() {
	defer doAfter(it.options.IntervalStartAndFinalizeJobs, it.startAndFinalizeJobs)

	for _, tenant := range it.tenants() {
		GoEach(ctxNone,
			func(ctx context.Context) { it.startDueJobs(ctx, tenant) },
			func(ctx context.Context) { it.finalizeFinishedJobs(ctx, tenant) },
			func(ctx context.Context) { it.finalizeCancelingJobs(ctx, tenant) },
		)
	}
}

func (it *engine) startDueJobs(ctx context.Context, tenant string) {
	log := loggerNew()
	var dueJobs []*Job
	DoTimeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.backend.listJobs(ctx, true, false, tenant, noPaging,
			JobFilter{}.WithStates(Pending).WithDue(true))
		if it.logErr(log, err) == nil {
			dueJobs = jobs
		}
	})
	{ // cancel rare duplicates and remnants of by-now-removed/disabled job-specs
		cancelJobs := map[CancellationReason][]*Job{}
		sort.Slice(dueJobs, func(i int, j int) bool { return !dueJobs[i].AutoScheduled })
		for i := 0; i < len(dueJobs); i++ {
			job := dueJobs[i]
			idx := sl.IdxWhere(dueJobs, func(j *Job) bool { // check if duplicate
				return j.Spec == job.Spec && cmp.Equal(j.Details, job.Details, cmpopts.IgnoreUnexported(), cmpopts.EquateEmpty())
			})
			var reason CancellationReason
			switch {
			case idx != i:
				reason = CancellationReasonDuplicate
			case job.spec == nil:
				reason = CancellationReasonSpecInvalidOrGone
			case job.HandlerID != job.spec.HandlerID || job.spec.Disabled:
				reason = CancellationReasonSpecChanged
			case job.spec.handler == nil && job.HandlerID == job.spec.HandlerID:
				reason = CancellationReasonJobTypeInvalidOrGone
			}
			if reason != "" {
				cancelJobs[reason] = append(cancelJobs[reason], job)
				dueJobs = append(dueJobs[:i], dueJobs[i+1:]...)
				i--
			}
		}
		for job, err := range it.cancelJobs(ctx, cancelJobs) {
			_ = it.logErr(log, err, job)
		}
	}
	GoItems(ctx, dueJobs, it.startDueJob,
		it.options.MaxConcurrentOps, 0 /* thus uses Job.Timeout() */)
}

func (it *engine) startDueJob(ctx context.Context, job *Job) {
	log := loggerNew()
	var err error
	if job.spec == nil {
		err = errNotFoundSpec(job.Spec)
	} else if job.spec.handler == nil {
		err = errNotFoundHandler(job.spec.ID, job.spec.HandlerID)
	}
	if it.logErr(log, err, job) != nil {
		return
	}

	// 1. handler.JobDetails
	timeStarted, jobCtx := timeNow(), job.ctx(ctx, "")
	if job.Details == nil {
		if job.Details, err = job.spec.defaultJobDetails(); it.logErr(log, err, job) != nil {
			return
		}
	}
	if job.Details, err = job.spec.handler.JobDetails(jobCtx); it.logErr(log, err, job) != nil {
		return
	}
	if _, err = handler(job.spec.HandlerID).wellTypedJobDetails(job.Details); it.logErr(log, err, job) != nil {
		return
	}

	// 2. handler.TaskDetails
	taskDetailsStream := make(chan []TaskDetails)
	go func() {
		defer close(taskDetailsStream)
		job.FinalTaskListReq, job.FinalTaskFilter = job.spec.handler.TaskDetails(jobCtx, taskDetailsStream, func(e error) error {
			if e != nil && err == nil {
				err = e
			}
			return err
		})
	}()
	_ = it.logErr(log, it.backend.transacted(ctx, func(ctx context.Context) error {
		var numTasks int
		for taskDetails := range taskDetailsStream {
			if err != nil { // don't `break` here: we need to drain the chan to close it, in the case of...
				continue // ...undisciplined `Handler.TaskDetails` impls (they should stop sending on err)
			}
			tasks := sl.To(taskDetails, func(details TaskDetails) *Task {
				if numTasks++; err == nil {
					_, err = handler(job.spec.HandlerID).wellTypedTaskDetails(details)
				}
				return &Task{
					Resource:        Resource{job.Tenant, job.ID + "_" + strconv.Itoa(numTasks)},
					Job:             job.ID,
					HandlerID:       job.HandlerID,
					State:           Pending,
					FinishTime:      nil,
					StartTime:       nil,
					ResourceVersion: 1,
					Details:         details,
				}
			})
			if len(tasks) > 0 && err == nil {
				err = it.backend.insertTasks(ctx, tasks...)
			}
		}
		if err == nil {
			job.State, job.StartTime, job.FinishTime, job.Info.DurationPrepInMinutes =
				Running, timeNow(), nil, ToPtr(timeNow().Sub(*timeStarted).Minutes())
			if it.logLifecycleEvents(false, nil, job, nil) {
				job.logger(log).Infof("marking %s '%s' job '%s' as %s (with %d tasks)", Pending, job.Spec, job.ID, job.State, numTasks)
			}
			err = it.backend.saveJob(ctx, job)
		}
		return err
	}), job)
}

func (it *engine) finalizeFinishedJobs(ctx context.Context, tenant string) {
	log := loggerNew()
	var runningJobs []*Job
	DoTimeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.backend.listJobs(ctx, true, false, tenant, noPaging,
			JobFilter{}.WithStates(Running))
		if it.logErr(log, err) == nil {
			runningJobs = jobs
		}
	})

	cancelJobs := map[CancellationReason][]*Job{}
	for reason, predicate := range map[CancellationReason]func(*Job) bool{
		CancellationReasonSpecInvalidOrGone: func(j *Job) bool {
			return j.spec == nil
		},
		CancellationReasonSpecChanged: func(j *Job) bool {
			return j.spec != nil && (j.HandlerID != j.spec.HandlerID || j.spec.Disabled)
		},
		CancellationReasonJobTypeInvalidOrGone: func(j *Job) bool {
			return j.spec != nil && j.spec.HandlerID == j.HandlerID && j.spec.handler == nil
		},
	} {
		jobsToCancel := sl.Where(runningJobs, predicate)
		cancelJobs[reason], runningJobs = jobsToCancel, sl.Without(runningJobs, jobsToCancel...)
	}
	for job, err := range it.cancelJobs(ctx, cancelJobs) {
		_ = it.logErr(log, err, job)
	}
	GoItems(ctx, runningJobs, it.finalizeFinishedJob,
		it.options.MaxConcurrentOps, 0 /* hence, Job.Timeout() */)
}

func (it *engine) finalizeFinishedJob(ctx context.Context, job *Job) {
	log := loggerNew()
	stillBusy, err := it.backend.findTask(ctx, false, false, job.Tenant,
		TaskFilter{}.WithStates(Pending, Running).WithJobs(job.ID))
	if it.logErr(log, err, job) != nil || stillBusy != nil {
		return
	}

	tasksFilter, tasksListReq, timeStarted := TaskFilter{}.WithJobs(job.ID), ListRequest{}, timeNow()
	if job.FinalTaskFilter != nil {
		tasksFilter = job.FinalTaskFilter.WithJobs(job.ID)
	}
	if job.FinalTaskListReq != nil {
		tasksListReq = *job.FinalTaskListReq
	}
	var tasksStream chan *Task
	abortStreaming := false
	job.Results, err = job.spec.handler.JobResults(job.ctx(ctx, ""), func() <-chan *Task {
		if tasksStream == nil {
			tasksStream = make(chan *Task, Clamp(0, 1024, tasksListReq.PageSize))
			go func(ctx context.Context) {
				defer close(tasksStream)
				for tasksListReq.PageToken = ""; !abortStreaming; { // bools dont need a mutex =)
					tasksPage, _, _, nextPageTok, err := it.backend.listTasks(ctx, false, false, job.Tenant,
						tasksListReq, tasksFilter)
					if it.logErr(log, err, job) != nil {
						return
					}
					for _, task := range tasksPage {
						if tasksStream <- task; abortStreaming {
							break
						}
					}
					if tasksListReq.PageToken = nextPageTok; tasksListReq.PageToken == "" {
						break
					}
				}
			}(ctx)
		}
		return tasksStream
	})
	abortStreaming = true
	if err == nil {
		_, err = handler(job.spec.HandlerID).wellTypedJobResults(job.Results)
	}
	if it.logErr(log, err, job) != nil {
		return
	}
	job.State, job.FinishTime, job.Info.DurationFinalizeInMinutes =
		Done, timeNow(), ToPtr(timeNow().Sub(*timeStarted).Minutes())

	if it.logLifecycleEvents(false, nil, job, nil) {
		job.logger(log).Infof("marking %s '%s' job '%s' as %s", Running, job.Spec, job.ID, Done)
	}
	if it.logErr(log, it.backend.transacted(ctx, func(ctx context.Context) error {
		err = it.backend.saveJob(ctx, job)
		if err == nil && job.AutoScheduled {
			// doing this right here upon job finalization (in the same
			// transaction) prevents concurrent duplicate job schedulings.
			err = it.scheduleJob(ctx, job.spec, job)
		}
		return err
	}), job) == nil && it.eventHandlers.OnJobExecuted != nil { // only count jobs that ran AND were stored
		if jobStats, err := it.JobStats(ctx, job.Resource); err == nil {
			it.eventHandlers.OnJobExecuted(job, jobStats)
		}
	}
}

func (it *engine) finalizeCancelingJobs(ctx context.Context, tenant string) {
	log := loggerNew()
	var cancelJobs []*Job
	DoTimeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.backend.listJobs(ctx, true, false, tenant, noPaging,
			JobFilter{}.WithStates(Cancelling))
		if it.logErr(log, err) == nil {
			cancelJobs = jobs
		}
	})
	GoItems(ctx, cancelJobs, it.finalizeCancelingJob,
		it.options.MaxConcurrentOps, TimeoutLong)
}

func (it *engine) finalizeCancelingJob(ctx context.Context, job *Job) {
	log := loggerNew()
	var numCanceled, numTasks int

	listReq := ListRequest{PageSize: 444}
	for {
		tasks, _, _, pageTok, err := it.backend.listTasks(ctx, false, false, job.Tenant, listReq,
			TaskFilter{}.WithJobs(job.ID).WithStates(Pending, Running))
		if it.logErr(log, err, job) != nil {
			return
		}
		listReq.PageToken, numTasks = pageTok, numTasks+len(tasks)
		for _, task := range tasks {
			if canceler := it.setTaskCanceler(task.ID, nil); canceler != nil {
				go canceler()
			} // this is optional/luxury, but nice if it succeeds due to (by chance) the Task being still Running & on this very same pod.

			task.job = job
			state := task.State
			task.State = Cancelled
			if it.logLifecycleEvents(true, nil, nil, task) {
				task.logger(log).Infof("marking %s task '%s' (of '%s' job '%s') as %s", state, task.ID, task.HandlerID, task.Job, task.State)
			}
			if nil == it.logErr(log, it.backend.saveTask(ctx, task), task) {
				numCanceled++
			}
		}
		if pageTok == "" {
			break
		}
	}
	if numTasks == numCanceled { // no more tasks left to cancel, now finalize
		job.State, job.FinishTime =
			Cancelled, timeNow()
		if it.logLifecycleEvents(false, nil, job, nil) {
			job.logger(log).Infof("marking %s '%s' job '%s' as %s", Cancelling, job.Spec, job.ID, Cancelled)
		}
		_ = it.logErr(log, it.backend.transacted(ctx, func(ctx context.Context) error {
			err := it.logErr(log, it.backend.saveJob(ctx, job), job)
			if err == nil && job.AutoScheduled && job.spec != nil && !job.spec.Disabled {
				// doing this right here upon job finalization (in the same
				// transaction) prevents concurrent duplicate job schedulings.
				err = it.scheduleJob(ctx, job.spec, job)
			}
			return err
		}), job)
	}
}

func (it *engine) ensureJobSchedules() {
	defer doAfter(it.options.IntervalEnsureJobSchedules, it.ensureJobSchedules)

	var jobSpecs []*JobSpec
	var mut sync.Mutex
	GoItems(ctxNone, it.tenants(), func(ctx context.Context, tenant string) {
		log := loggerNew()
		tenantJobSpecs, err := it.backend.listJobSpecs(ctx, tenant,
			JobSpecFilter{}.WithDisabled(false).WithEnabledSchedules())
		if it.logErr(log, err) == nil {
			mut.Lock()
			defer mut.Unlock()
			jobSpecs = append(jobSpecs, tenantJobSpecs...)
		}
	}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
	GoItems(ctxNone, jobSpecs, it.ensureJobSpecScheduled,
		it.options.MaxConcurrentOps, it.options.TimeoutShort)
}

func (it *engine) ensureJobSpecScheduled(ctx context.Context, jobSpec *JobSpec) {
	log := loggerNew()

	latest, err := it.backend.findJob(ctx, false, false, jobSpec.Tenant, // defaults to sorted descending by due_time
		JobFilter{}.WithJobSpecs(jobSpec.ID).WithAutoScheduled(true))
	if it.logErr(log, err, jobSpec) != nil || (latest != nil && // still busy? then no scheduling needed here & now
		(latest.State == Running || latest.State == Cancelling)) {
		return
	}
	if latest != nil {
		latest.spec = jobSpec
	}
	if latest == nil || latest.State != Pending {
		_ = it.logErr(log, it.scheduleJob(ctx, jobSpec, latest), jobSpec)
	} else if latest.DueTime.After(*timeNow()) { // verify the Pending job's future due_time against the current `jobSpec.Schedules` in case the latter changed after the former was scheduled
		var after *time.Time
		lastDone, err := it.backend.findJob(ctx, false, false, jobSpec.Tenant,
			JobFilter{}.WithJobSpecs(jobSpec.ID).WithStates(Done, Cancelled))
		if it.logErr(log, err, latest) != nil {
			return
		}
		if lastDone != nil {
			after = firstNonNil(lastDone.FinishTime, lastDone.StartTime, &lastDone.DueTime)
		}
		dueTime := jobSpec.findClosestToNowSchedulableTimeSince(after, true)
		if dueTime == nil { // jobSpec or all its Schedules were Disabled since this Pending Job was scheduled
			for job, err := range it.cancelJobs(ctx, map[CancellationReason][]*Job{
				CancellationReasonSpecChanged: {latest},
			}) {
				_ = it.logErr(log, err, job)
			}
			return
		}
		if (!jobSpec.ok(latest.DueTime)) || !dueTime.Equal(latest.DueTime) {
			if it.logLifecycleEvents(false, nil, latest, nil) {
				latest.logger(log).Infof("updating outdated scheduled due_time of '%s' job '%s' from '%s' to '%s'", jobSpec.ID, latest.ID, latest.DueTime, dueTime)
			}
			latest.DueTime = *dueTime
			_ = it.logErr(log, it.backend.saveJob(ctx, latest), latest)
		}
	}
}

func (it *engine) scheduleJob(ctx context.Context, jobSpec *JobSpec, last *Job) error {
	if jobSpec.Disabled || jobSpec.handler == nil {
		return nil
	}
	var lastTime *time.Time
	if last != nil {
		lastTime = firstNonNil(last.FinishTime, last.StartTime, &last.DueTime)
	}
	dueTime := jobSpec.findClosestToNowSchedulableTimeSince(lastTime, true)
	if dueTime == nil { // means currently no non-Disabled `Schedules`, so don't schedule anything
		return nil
	}
	_, err := it.createJob(ctx, jobSpec, "", *dueTime, nil, last, true)
	return err
}

func (it *engine) deleteStorageExpiredJobs() {
	defer doAfter(it.options.IntervalDeleteStorageExpiredJobs, it.deleteStorageExpiredJobs)

	for _, tenant := range it.tenants() {
		DoTimeout(ctxNone, it.options.TimeoutShort, func(ctx context.Context) {
			log := loggerNew()
			jobSpecs, err := it.backend.listJobSpecs(ctx, tenant,
				JobSpecFilter{}.WithStorageExpiry(true))
			if it.logErr(log, err) != nil {
				return
			}
			for _, jobSpec := range jobSpecs {
				it.deleteStorageExpiredJobsForSpec(ctx, jobSpec)
			}
		})
	}
}

func (it *engine) deleteStorageExpiredJobsForSpec(ctx context.Context, jobSpec *JobSpec) {
	log := loggerNew()
	jobsToDelete, _, _, err := it.backend.listJobs(ctx, true, false, jobSpec.Tenant, noPaging,
		JobFilter{}.WithStates(Done, Cancelled).WithJobSpecs(jobSpec.ID).
			WithFinishedBefore(timeNow().AddDate(0, 0, -jobSpec.DeleteAfterDays)))
	if it.logErr(log, err, jobSpec) != nil {
		return
	}

	for _, job := range jobsToDelete {
		if it.logLifecycleEvents(false, nil, job, nil) {
			job.logger(log).Infof("deleting %s '%s' job '%s' and its tasks", job.State, jobSpec.ID, job.ID)
		}
		_ = it.logErr(log, it.backend.transacted(ctx, func(ctx context.Context) error {
			err := it.backend.deleteTasks(ctx, jobSpec.Tenant, TaskFilter{}.WithJobs(job.ID))
			if err == nil {
				err = it.backend.deleteJobs(ctx, jobSpec.Tenant, JobFilter{}.WithIDs(job.ID))
			}
			return err
		}), job)
	}
}

// A died task is one whose runner died between its start and its finishing or orderly timeout.
// It's found in the DB as still RUNNING despite its timeout moment being over a minute ago:
func (it *engine) expireOrRetryDeadTasks() {
	defer doAfter(it.options.IntervalExpireOrRetryDeadTasks, it.expireOrRetryDeadTasks)

	currentlyRunning, mut := map[*JobSpec][]*Job{}, sync.Mutex{} // gather candidate jobs for task selection
	GoItems(ctxNone, it.tenants(), func(ctx context.Context, tenant string) {
		log := loggerNew()
		jobs, _, _, err := it.backend.listJobs(ctx, true, false, tenant, noPaging,
			JobFilter{}.WithStates(Running))
		if it.logErr(log, err) != nil || len(jobs) == 0 {
			return
		}

		mut.Lock()
		defer mut.Unlock()
		for _, job := range jobs {
			currentlyRunning[job.spec] = append(currentlyRunning[job.spec], job)
		}
	}, it.options.MaxConcurrentOps, it.options.TimeoutShort)

	GoItems(ctxNone, sl.Keys(currentlyRunning), func(ctx context.Context, js *JobSpec) {
		it.expireOrRetryDeadTasksForSpec(ctx, js, currentlyRunning[js])
	}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
}

func (it *engine) expireOrRetryDeadTasksForSpec(ctx context.Context, jobSpec *JobSpec, runningJobs []*Job) {
	log := loggerNew()
	specDead, taskFilter := jobSpec == nil || jobSpec.Disabled || jobSpec.handler == nil, TaskFilter{}.
		WithJobs(sl.To(runningJobs, func(v *Job) string { return v.ID })...)
	if !specDead { // the usual case.
		taskFilter = taskFilter.WithStates(Running).WithStartedBefore(timeNow().Add(-(jobSpec.Timeouts.TaskRun + time.Minute)))
	} else { //  the rare edge case: un-Done tasks still in DB for old now-disabled-or-deleted-from-config job spec
		taskFilter = taskFilter.WithStates(Running, Pending)
	}
	deadTasks, _, _, _, err := it.backend.listTasks(ctx, false, false, runningJobs[0].Tenant, noPaging, taskFilter)
	if it.logErr(log, err, jobSpec) != nil {
		return
	}
	for _, task := range deadTasks {
		if specDead {
			task.State = Cancelled
			if len(task.Attempts) > 0 && task.Attempts[0].Err == nil {
				task.Attempts[0].Err = context.Canceled
			}
		} else if (!task.markForRetryOrAsFailed(jobSpec)) && len(task.Attempts) > 0 && task.Attempts[0].Err == nil {
			task.Attempts[0].Err = context.DeadlineExceeded
		}
		if it.logLifecycleEvents(true, jobSpec, nil, task) {
			task.logger(log).Infof("marking dead (state %s after timeout) task '%s' (of '%s' job '%s') as %s", Running, task.ID, jobSpec.ID, task.Job, task.State)
		}
		_ = it.logErr(log, it.backend.saveTask(ctx, task), task)
	}
}

func (it *engine) runTasks() {
	callAgainIn := it.options.IntervalRunTasks // prolonged below if zero action right now, skipped if plenty action right now
	defer func() { doAfter(callAgainIn, it.runTasks) }()

	var soonestDue *time.Time   // for adaptive slowdown at the end
	var manualJobsPossible bool // dito
	// fetch some Pending tasks
	pendingTasks, mut := []*Task{}, sync.Mutex{}
	GoItems(ctxNone, it.tenants(), func(ctx context.Context, tenant string) {
		log := loggerNew()
		tasks, _, _, _, err := it.backend.listTasks(ctx, true, false, tenant, ListRequest{PageSize: it.options.FetchTasksToRunPerTenant},
			TaskFilter{}.WithStates(Pending))
		if it.logErr(log, err) != nil {
			return
		}
		mut.Lock()
		defer mut.Unlock()
		pendingTasks = append(pendingTasks, tasks...)

		if len(pendingTasks) == 0 && !manualJobsPossible {
			if manualJobsPossible = it.manualJobsPossible(ctx, tenant); !manualJobsPossible {
				upcomingJob, err := it.backend.findJob(ctx, false, false, tenant,
					JobFilter{}.WithStates(Pending))
				if it.logErr(log, err) == nil && upcomingJob != nil &&
					(soonestDue == nil || upcomingJob.DueTime.Before(*soonestDue)) {
					soonestDue = ToPtr(upcomingJob.DueTime.In(Timezone))
				}
			}
		}
	}, it.options.MaxConcurrentOps, it.options.TimeoutShort)

	// ...then run them
	GoItems(ctxNone, pendingTasks, func(ctx context.Context, task *Task) {
		_ = it.runTask(ctx, task)
	}, it.options.MaxConcurrentOps, 0 /* hence, task.Timeout() */)

	if len(pendingTasks) > 0 { // times are busy: fetch more tasks sooner!
		callAgainIn = 123 * time.Millisecond // but allow for other pods to get through and pick stuff too
	} else if len(pendingTasks) == 0 { // times are quiet: can wait twice as long to reduce traffic...
		callAgainIn = 2 * it.options.IntervalRunTasks // but shouldn't wait too long either: once a Job has started, it shouldn't hang inactive for minutes. plus there may be manual job creations
		if soonestDue != nil && !manualJobsPossible { // best case: we have a soonest-upcoming-job-due-time...
			if soonestDue.After(*timeNow()) { // then we can wait right until just then!
				dur := soonestDue.Sub(*timeNow()) // except, not. an interim config change might mean a schedule change too, so:
				callAgainIn = If(dur < it.options.IntervalEnsureJobSchedules, dur,
					it.options.IntervalEnsureJobSchedules) + it.options.IntervalStartAndFinalizeJobs
			} else { // the soonest was actually overdue, so tasks to do any-second-now
				callAgainIn = it.options.IntervalRunTasks
			}
		}
	}
}

func (it *engine) runTask(ctx context.Context, task *Task) error {
	log, timeStarted := loggerNew(), timeNow()
	ctxOrig := ctx
	ctx, done := context.WithCancel(ctx)
	if oldCanceller := it.setTaskCanceler(task.ID, done); oldCanceller != nil {
		oldCanceller() // should never be the case, but let's be principled & clean...
	}
	defer func() {
		if done = it.setTaskCanceler(task.ID, nil); done != nil {
			done()
		} // else: already cancelled by concurrent `finalizeCancelingJobs` call
	}()

	taskJobSpecOrType := task.HandlerID
	if task.job != nil {
		taskJobSpecOrType = task.job.Spec
	}
	// first, attempt to reserve task for running vs. other pods
	alreadyCancelled := task.job == nil || task.job.State == Cancelled || task.job.State == Cancelling ||
		task.job.spec == nil || task.job.spec.Disabled || task.job.spec.handler == nil ||
		task.HandlerID != task.job.spec.HandlerID || task.job.HandlerID != task.job.spec.HandlerID
	oldTaskState := task.State
	task.State, task.FinishTime, task.Attempts =
		If(alreadyCancelled, Cancelled, Running), nil, append([]*TaskAttempt{{Time: *timeNow()}}, task.Attempts...)
	if task.StartTime == nil {
		task.StartTime = timeNow()
	}
	if it.logLifecycleEvents(true, nil, nil, task) {
		task.logger(log).Infof("marking %s task '%s' (of '%s' job '%s') as %s", oldTaskState, task.ID, taskJobSpecOrType, task.Job, task.State)
	}
	if err := it.logErr(log, it.backend.saveTask(ctx, task), task); err != nil {
		return err
	}

	switch {
	case task.job == nil:
		task.Attempts[0].Err = errNotFoundJob(task.Job)
	case task.job.spec == nil:
		task.Attempts[0].Err = errNotFoundSpec(task.job.Spec)
	case task.job.spec.handler == nil:
		task.Attempts[0].Err = errNotFoundHandler(task.job.Spec, task.job.spec.HandlerID)
	case !alreadyCancelled: // now run it
		task.Results, task.Attempts[0].Err = task.job.spec.handler.TaskResults(task.job.ctx(ctx, task.ID), task.Details)
		if task.Attempts[0].Err == nil {
			_, task.Attempts[0].Err = handler(task.job.spec.HandlerID).wellTypedTaskResults(task.Results)
		}
	}

	task.State, task.FinishTime =
		If(alreadyCancelled, Cancelled, Done), timeNow()
	ctxErr := ctx.Err()
	if ctxErr != nil && errors.Is(ctxErr, context.Canceled) {
		task.State = Cancelled
	} else if (!alreadyCancelled) &&
		((ctxErr != nil && errors.Is(ctxErr, context.DeadlineExceeded)) ||
			((task.Attempts[0].Err != nil) && (task.job.spec.handler != nil) &&
				task.job.spec.handler.IsTaskErrRetryable(task.Attempts[0].Err))) {
		_ = task.markForRetryOrAsFailed(task.job.spec)
	}
	if task.Attempts[0].Err == nil {
		task.Attempts[0].Err = ctxErr
	}
	if _ = it.logErr(log, task.Attempts[0].Err, task); it.logLifecycleEvents(true, nil, nil, task) {
		task.logger(log).Infof("marking just-%s %s task '%s' (of '%s' job '%s') as %s", If(task.Attempts[0].Err != nil, "failed", "finished"), Running, task.ID, taskJobSpecOrType, task.Job, task.State)
	}
	err := it.logErr(log, it.backend.saveTask(ctxOrig, task), task)
	if err == nil && it.eventHandlers.OnTaskExecuted != nil { // only count tasks that actually ran (failed or not) AND were stored
		it.eventHandlers.OnTaskExecuted(task, timeNow().Sub(*timeStarted))
	}
	return err
}

func (it *engine) manualJobsPossible(ctx context.Context, tenant string) bool {
	log := loggerNew()
	jobSpecManual, err := it.backend.findJobSpec(ctx, tenant,
		JobSpecFilter{}.WithAllowManualJobs(true))
	return it.logErr(log, err) != nil || jobSpecManual != nil
}

func (it *engine) setTaskCanceler(id string, cancel context.CancelFunc) (previous context.CancelFunc) {
	it.taskCancelersMut.Lock()
	defer it.taskCancelersMut.Unlock()
	previous = it.taskCancelers[id]
	if cancel == nil {
		delete(it.taskCancelers, id)
	} else {
		it.taskCancelers[id] = cancel
	}
	return
}
