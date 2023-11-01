package jobs

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
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

func (it *engine) startAndFinalizeJobRuns() {
	defer doAfter(it.options.IntervalStartAndFinalizeJobs, it.startAndFinalizeJobRuns)

	GoEach(ctxNone,
		func(ctx context.Context) { it.startDueJobs(ctx) },
		func(ctx context.Context) { it.finalizeFinishedJobs(ctx) },
		func(ctx context.Context) { it.finalizeCancelingJobs(ctx) },
	)
}

func (it *engine) startDueJobs(ctx context.Context) {
	log := loggerNew()
	var dueJobs []*JobRun
	DoTimeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.backend.listJobRuns(ctx, true, false, noPaging,
			JobRunFilter{}.WithStates(Pending).WithDue(true))
		if it.logErr(log, err) == nil {
			dueJobs = jobs
		}
	})
	{ // cancel rare duplicates and remnants of by-now-removed/disabled job-defs
		cancelJobs := map[CancellationReason][]*JobRun{}
		sort.Slice(dueJobs, func(i int, j int) bool { return !dueJobs[i].AutoScheduled })
		for i := 0; i < len(dueJobs); i++ {
			job := dueJobs[i]
			idx := sl.IdxWhere(dueJobs, func(j *JobRun) bool { // check if duplicate
				return j.JobDefId == job.JobDefId && cmp.Equal(j.Details, job.Details, cmpopts.IgnoreUnexported(), cmpopts.EquateEmpty())
			})
			var reason CancellationReason
			switch {
			case idx != i:
				reason = CancellationReasonDuplicate
			case job.jobDef == nil:
				reason = CancellationReasonDefInvalidOrGone
			case job.HandlerId != job.jobDef.HandlerId || job.jobDef.Disabled:
				reason = CancellationReasonDefChanged
			case job.jobDef.handler == nil && job.HandlerId == job.jobDef.HandlerId:
				reason = CancellationReasonJobTypeInvalidOrGone
			}
			if reason != "" {
				cancelJobs[reason] = append(cancelJobs[reason], job)
				dueJobs = append(dueJobs[:i], dueJobs[i+1:]...)
				i--
			}
		}
		for job, err := range it.cancelJobRuns(ctx, cancelJobs) {
			_ = it.logErr(log, err, job)
		}
	}
	GoItems(ctx, dueJobs, it.startDueJob,
		it.options.MaxConcurrentOps, 0 /* thus uses Job.Timeout() */)
}

func (it *engine) startDueJob(ctx context.Context, job *JobRun) {
	log := loggerNew()
	var err error
	if job.jobDef == nil {
		err = errNotFoundJobDef(job.JobDefId)
	} else if job.jobDef.handler == nil {
		err = errNotFoundHandler(job.jobDef.Id, job.jobDef.HandlerId)
	}
	if it.logErr(log, err, job) != nil {
		return
	}

	// 1. handler.JobDetails
	timeStarted, jobCtx := timeNow(), job.ctx(ctx, "")
	if job.Details == nil {
		if job.Details, err = job.jobDef.defaultJobDetails(); it.logErr(log, err, job) != nil {
			return
		}
	}
	if job.Details, err = job.jobDef.handler.JobDetails(jobCtx); it.logErr(log, err, job) != nil {
		return
	}
	if _, err = handler(job.jobDef.HandlerId).wellTypedJobDetails(job.Details); it.logErr(log, err, job) != nil {
		return
	}

	// 2. handler.TaskDetails
	taskDetailsStream := make(chan []TaskDetails)
	go func() {
		defer close(taskDetailsStream)
		job.FinalTaskListReq, job.FinalTaskFilter = job.jobDef.handler.TaskDetails(jobCtx, taskDetailsStream, func(e error) error {
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
					_, err = handler(job.jobDef.HandlerId).wellTypedTaskDetails(details)
				}
				return &Task{
					Resource:        Resource{job.Id + "_" + strconv.Itoa(numTasks)},
					JobRunId:        job.Id,
					HandlerId:       job.HandlerId,
					State:           Pending,
					FinishTime:      nil,
					StartTime:       nil,
					ResourceVersion: 1,
					Details:         details,
				}
			})
			if len(tasks) > 0 && err == nil {
				err = it.backend.insertJobTasks(ctx, tasks...)
			}
		}
		if err == nil {
			job.State, job.StartTime, job.FinishTime, job.Info.DurationPrepInMinutes =
				Running, timeNow(), nil, ToPtr(timeNow().Sub(*timeStarted).Minutes())
			if it.logLifecycleEvents(nil, job, nil) {
				job.logger(log).Infof("marking %s '%s' job '%s' as %s (with %d tasks)", Pending, job.JobDefId, job.Id, job.State, numTasks)
			}
			err = it.backend.saveJobRun(ctx, job)
		}
		return err
	}), job)
}

func (it *engine) finalizeFinishedJobs(ctx context.Context) {
	log := loggerNew()
	var runningJobs []*JobRun
	DoTimeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.backend.listJobRuns(ctx, true, false, noPaging,
			JobRunFilter{}.WithStates(Running))
		if it.logErr(log, err) == nil {
			runningJobs = jobs
		}
	})

	cancelJobs := map[CancellationReason][]*JobRun{}
	for reason, predicate := range map[CancellationReason]func(*JobRun) bool{
		CancellationReasonDefInvalidOrGone: func(j *JobRun) bool {
			return j.jobDef == nil
		},
		CancellationReasonDefChanged: func(j *JobRun) bool {
			return j.jobDef != nil && (j.HandlerId != j.jobDef.HandlerId || j.jobDef.Disabled)
		},
		CancellationReasonJobTypeInvalidOrGone: func(j *JobRun) bool {
			return j.jobDef != nil && j.jobDef.HandlerId == j.HandlerId && j.jobDef.handler == nil
		},
	} {
		jobsToCancel := sl.Where(runningJobs, predicate)
		cancelJobs[reason], runningJobs = jobsToCancel, sl.Without(runningJobs, jobsToCancel...)
	}
	for job, err := range it.cancelJobRuns(ctx, cancelJobs) {
		_ = it.logErr(log, err, job)
	}
	GoItems(ctx, runningJobs, it.finalizeFinishedJob,
		it.options.MaxConcurrentOps, 0 /* hence, Job.Timeout() */)
}

func (it *engine) finalizeFinishedJob(ctx context.Context, job *JobRun) {
	log := loggerNew()
	stillBusy, err := it.backend.findJobTask(ctx, false, false,
		JobTaskFilter{}.WithStates(Pending, Running).WithJobRuns(job.Id))
	if it.logErr(log, err, job) != nil || stillBusy != nil {
		return
	}

	tasksFilter, tasksListReq, timeStarted := JobTaskFilter{}.WithJobRuns(job.Id), ListRequest{}, timeNow()
	if job.FinalTaskFilter != nil {
		tasksFilter = job.FinalTaskFilter.WithJobRuns(job.Id)
	}
	if job.FinalTaskListReq != nil {
		tasksListReq = *job.FinalTaskListReq
	}
	var tasksStream chan *Task
	abortStreaming := false
	job.Results, err = job.jobDef.handler.JobResults(job.ctx(ctx, ""), func() <-chan *Task {
		if tasksStream == nil {
			tasksStream = make(chan *Task, Clamp(0, 1024, tasksListReq.PageSize))
			go func(ctx context.Context) {
				defer close(tasksStream)
				for tasksListReq.PageToken = ""; !abortStreaming; { // bools dont need a mutex =)
					tasksPage, _, _, nextPageTok, err := it.backend.listJobTasks(ctx, false, false,
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
		_, err = handler(job.jobDef.HandlerId).wellTypedJobResults(job.Results)
	}
	if it.logErr(log, err, job) != nil {
		return
	}
	job.State, job.FinishTime, job.Info.DurationFinalizeInMinutes =
		Done, timeNow(), ToPtr(timeNow().Sub(*timeStarted).Minutes())

	if it.logLifecycleEvents(nil, job, nil) {
		job.logger(log).Infof("marking %s '%s' job '%s' as %s", Running, job.JobDefId, job.Id, Done)
	}
	if it.logErr(log, it.backend.transacted(ctx, func(ctx context.Context) error {
		err = it.backend.saveJobRun(ctx, job)
		if err == nil && job.AutoScheduled {
			// doing this right here upon job finalization (in the same
			// transaction) prevents concurrent duplicate job schedulings.
			err = it.scheduleJob(ctx, job.jobDef, job)
		}
		return err
	}), job) == nil && it.eventHandlers.onJobRunExecuted != nil { // only count jobs that ran AND were stored
		if jobStats, err := it.Stats(ctx, job.Resource); err == nil {
			it.eventHandlers.onJobRunExecuted(job, jobStats)
		}
	}
}

func (it *engine) finalizeCancelingJobs(ctx context.Context) {
	log := loggerNew()
	var cancelJobs []*JobRun
	DoTimeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.backend.listJobRuns(ctx, true, false, noPaging,
			JobRunFilter{}.WithStates(JobRunCancelling))
		if it.logErr(log, err) == nil {
			cancelJobs = jobs
		}
	})
	GoItems(ctx, cancelJobs, it.finalizeCancelingJob,
		it.options.MaxConcurrentOps, TimeoutLong)
}

func (it *engine) finalizeCancelingJob(ctx context.Context, job *JobRun) {
	log := loggerNew()
	var numCanceled, numTasks int

	listReq := ListRequest{PageSize: 444}
	for {
		tasks, _, _, pageTok, err := it.backend.listJobTasks(ctx, false, false, listReq,
			JobTaskFilter{}.WithJobRuns(job.Id).WithStates(Pending, Running))
		if it.logErr(log, err, job) != nil {
			return
		}
		listReq.PageToken, numTasks = pageTok, numTasks+len(tasks)
		for _, task := range tasks {
			if canceler := it.setTaskCanceler(task.Id, nil); canceler != nil {
				go canceler()
			} // this is optional/luxury, but nice if it succeeds due to (by chance) the Task being still Running & on this very same pod.

			task.jobRun = job
			state := task.State
			task.State = Cancelled
			if it.logLifecycleEvents(nil, nil, task) {
				task.logger(log).Infof("marking %s task '%s' (of '%s' job '%s') as %s", state, task.Id, task.HandlerId, task.JobRunId, task.State)
			}
			if nil == it.logErr(log, it.backend.saveJobTask(ctx, task), task) {
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
		if it.logLifecycleEvents(nil, job, nil) {
			job.logger(log).Infof("marking %s '%s' job '%s' as %s", JobRunCancelling, job.JobDefId, job.Id, Cancelled)
		}
		_ = it.logErr(log, it.backend.transacted(ctx, func(ctx context.Context) error {
			err := it.logErr(log, it.backend.saveJobRun(ctx, job), job)
			if err == nil && job.AutoScheduled && job.jobDef != nil && !job.jobDef.Disabled {
				// doing this right here upon job finalization (in the same
				// transaction) prevents concurrent duplicate job schedulings.
				err = it.scheduleJob(ctx, job.jobDef, job)
			}
			return err
		}), job)
	}
}

func (it *engine) ensureJobRunSchedules() {
	defer doAfter(it.options.IntervalEnsureJobSchedules, it.ensureJobRunSchedules)

	var jobDefs []*JobDef
	{
		var err error
		log := loggerNew()
		jobDefs, err = it.backend.listJobDefs(ctxNone,
			JobDefFilter{}.WithDisabled(false).WithEnabledSchedules())
		it.logErr(log, err)
	}
	GoItems(ctxNone, jobDefs, it.ensureJobDefScheduled,
		it.options.MaxConcurrentOps, it.options.TimeoutShort)
}

func (it *engine) ensureJobDefScheduled(ctx context.Context, jobDef *JobDef) {
	log := loggerNew()

	latest, err := it.backend.findJobRun(ctx, false, false, // defaults to sorted descending by due_time
		JobRunFilter{}.WithJobDefs(jobDef.Id).WithAutoScheduled(true))
	if it.logErr(log, err, jobDef) != nil || (latest != nil && // still busy? then no scheduling needed here & now
		(latest.State == Running || latest.State == JobRunCancelling)) {
		return
	}
	if latest != nil {
		latest.jobDef = jobDef
	}
	if latest == nil || latest.State != Pending {
		_ = it.logErr(log, it.scheduleJob(ctx, jobDef, latest), jobDef)
	} else if latest.DueTime.After(*timeNow()) { // verify the Pending job's future due_time against the current `jobDef.Schedules` in case the latter changed after the former was scheduled
		var after *time.Time
		lastDone, err := it.backend.findJobRun(ctx, false, false,
			JobRunFilter{}.WithJobDefs(jobDef.Id).WithStates(Done, Cancelled))
		if it.logErr(log, err, latest) != nil {
			return
		}
		if lastDone != nil {
			after = firstNonNil(lastDone.FinishTime, lastDone.StartTime, &lastDone.DueTime)
		}
		dueTime := jobDef.findClosestToNowSchedulableTimeSince(after, true)
		if dueTime == nil { // jobDef or all its Schedules were Disabled since this Pending Job was scheduled
			for job, err := range it.cancelJobRuns(ctx, map[CancellationReason][]*JobRun{
				CancellationReasonDefChanged: {latest},
			}) {
				_ = it.logErr(log, err, job)
			}
			return
		}
		if (!jobDef.ok(latest.DueTime)) || !dueTime.Equal(latest.DueTime) {
			if it.logLifecycleEvents(nil, latest, nil) {
				latest.logger(log).Infof("updating outdated scheduled due_time of '%s' job '%s' from '%s' to '%s'", jobDef.Id, latest.Id, latest.DueTime, dueTime)
			}
			latest.DueTime = *dueTime
			_ = it.logErr(log, it.backend.saveJobRun(ctx, latest), latest)
		}
	}
}

func (it *engine) scheduleJob(ctx context.Context, jobDef *JobDef, last *JobRun) error {
	if jobDef.Disabled || jobDef.handler == nil {
		return nil
	}
	var lastTime *time.Time
	if last != nil {
		lastTime = firstNonNil(last.FinishTime, last.StartTime, &last.DueTime)
	}
	dueTime := jobDef.findClosestToNowSchedulableTimeSince(lastTime, true)
	if dueTime == nil { // means currently no non-Disabled `Schedules`, so don't schedule anything
		return nil
	}
	_, err := it.createJobRun(ctx, jobDef, "", *dueTime, nil, last, true)
	return err
}

func (it *engine) deleteStorageExpiredJobRuns() {
	defer doAfter(it.options.IntervalDeleteStorageExpiredJobs, it.deleteStorageExpiredJobRuns)

	DoTimeout(ctxNone, it.options.TimeoutShort, func(ctx context.Context) {
		log := loggerNew()
		jobDefs, err := it.backend.listJobDefs(ctx, JobDefFilter{}.WithStorageExpiry(true))
		if it.logErr(log, err) != nil {
			return
		}
		for _, jobDef := range jobDefs {
			it.deleteStorageExpiredJobsForDef(ctx, jobDef)
		}
	})
}

func (it *engine) deleteStorageExpiredJobsForDef(ctx context.Context, jobDef *JobDef) {
	log := loggerNew()
	jobsToDelete, _, _, err := it.backend.listJobRuns(ctx, true, false, noPaging,
		JobRunFilter{}.WithStates(Done, Cancelled).WithJobDefs(jobDef.Id).
			WithFinishedBefore(timeNow().AddDate(0, 0, -jobDef.DeleteAfterDays)))
	if it.logErr(log, err, jobDef) != nil {
		return
	}

	for _, job := range jobsToDelete {
		if it.logLifecycleEvents(nil, job, nil) {
			job.logger(log).Infof("deleting %s '%s' job '%s' and its tasks", job.State, jobDef.Id, job.Id)
		}
		_ = it.logErr(log, it.backend.transacted(ctx, func(ctx context.Context) error {
			err := it.backend.deleteJobTasks(ctx, JobTaskFilter{}.WithJobRuns(job.Id))
			if err == nil {
				err = it.backend.deleteJobRuns(ctx, JobRunFilter{}.WithIds(job.Id))
			}
			return err
		}), job)
	}
}

// A died task is one whose runner died between its start and its finishing or orderly timeout.
// It's found in the DB as still RUNNING despite its timeout moment being over a minute ago:
func (it *engine) expireOrRetryDeadJobTasks() {
	defer doAfter(it.options.IntervalExpireOrRetryDeadTasks, it.expireOrRetryDeadJobTasks)

	currentlyRunning := map[*JobDef][]*JobRun{} // gather candidate jobs for task selection
	{
		log := loggerNew()
		jobs, _, _, err := it.backend.listJobRuns(ctxNone, true, false, noPaging,
			JobRunFilter{}.WithStates(Running))
		if it.logErr(log, err) != nil || len(jobs) == 0 {
			return
		}

		for _, job := range jobs {
			currentlyRunning[job.jobDef] = append(currentlyRunning[job.jobDef], job)
		}
	}

	GoItems(ctxNone, sl.Keys(currentlyRunning), func(ctx context.Context, js *JobDef) {
		it.expireOrRetryDeadTasksForDef(ctx, js, currentlyRunning[js])
	}, it.options.MaxConcurrentOps, it.options.TimeoutShort)
}

func (it *engine) expireOrRetryDeadTasksForDef(ctx context.Context, jobDef *JobDef, runningJobs []*JobRun) {
	log := loggerNew()
	defDead, taskFilter := jobDef == nil || jobDef.Disabled || jobDef.handler == nil, JobTaskFilter{}.
		WithJobRuns(sl.To(runningJobs, func(v *JobRun) string { return v.Id })...)
	if !defDead { // the usual case.
		taskFilter = taskFilter.WithStates(Running).WithStartedBefore(timeNow().Add(-(jobDef.Timeouts.TaskRun + time.Minute)))
	} else { //  the rare edge case: un-Done tasks still in DB for old now-disabled-or-deleted-from-config job def
		taskFilter = taskFilter.WithStates(Running, Pending)
	}
	deadTasks, _, _, _, err := it.backend.listJobTasks(ctx, false, false, noPaging, taskFilter)
	if it.logErr(log, err, jobDef) != nil {
		return
	}
	for _, task := range deadTasks {
		if defDead {
			task.State = Cancelled
			if len(task.Attempts) > 0 && task.Attempts[0].Err == nil {
				task.Attempts[0].Err = context.Canceled
			}
		} else if (!task.markForRetryOrAsFailed(jobDef)) && len(task.Attempts) > 0 && task.Attempts[0].Err == nil {
			task.Attempts[0].Err = context.DeadlineExceeded
		}
		if it.logLifecycleEvents(jobDef, nil, task) {
			task.logger(log).Infof("marking dead (state %s after timeout) task '%s' (of '%s' job '%s') as %s", Running, task.Id, jobDef.Id, task.JobRunId, task.State)
		}
		_ = it.logErr(log, it.backend.saveJobTask(ctx, task), task)
	}
}

func (it *engine) runJobTasks() {
	defer func() { doAfter(it.options.IntervalRunTasks, it.runJobTasks) }()

	var pendingTasks []*Task
	{
		var err error
		log := loggerNew()
		pendingTasks, _, _, _, err = it.backend.listJobTasks(ctxNone, true, false, ListRequest{PageSize: it.options.FetchTasksToRun},
			JobTaskFilter{}.WithStates(Pending))
		if it.logErr(log, err) != nil {
			return
		}
	}

	// ...then run them
	GoItems(ctxNone, pendingTasks, func(ctx context.Context, task *Task) {
		_ = it.runTask(ctx, task)
	}, it.options.MaxConcurrentOps, 0 /* hence, task.Timeout() */)
}

func (it *engine) runTask(ctx context.Context, task *Task) error {
	log, timeStarted := loggerNew(), timeNow()
	ctxOrig := ctx
	ctx, done := context.WithCancel(ctx)
	if oldCanceller := it.setTaskCanceler(task.Id, done); oldCanceller != nil {
		oldCanceller() // should never be the case, but let's be principled & clean...
	}
	defer func() {
		if done = it.setTaskCanceler(task.Id, nil); done != nil {
			done()
		} // else: already cancelled by concurrent `finalizeCancelingJobs` call
	}()

	taskJobDefOrType := task.HandlerId
	if task.jobRun != nil {
		taskJobDefOrType = task.jobRun.JobDefId
	}
	// first, attempt to reserve task for running vs. other pods
	alreadyCancelled := task.jobRun == nil || task.jobRun.State == Cancelled || task.jobRun.State == JobRunCancelling ||
		task.jobRun.jobDef == nil || task.jobRun.jobDef.Disabled || task.jobRun.jobDef.handler == nil ||
		task.HandlerId != task.jobRun.jobDef.HandlerId || task.jobRun.HandlerId != task.jobRun.jobDef.HandlerId
	oldTaskState := task.State
	task.State, task.FinishTime, task.Attempts =
		If(alreadyCancelled, Cancelled, Running), nil, append([]*TaskAttempt{{Time: *timeNow()}}, task.Attempts...)
	if task.StartTime == nil {
		task.StartTime = timeNow()
	}
	if it.logLifecycleEvents(nil, nil, task) {
		task.logger(log).Infof("marking %s task '%s' (of '%s' job '%s') as %s", oldTaskState, task.Id, taskJobDefOrType, task.JobRunId, task.State)
	}
	if err := it.logErr(log, it.backend.saveJobTask(ctx, task), task); err != nil {
		return err
	}

	switch {
	case task.jobRun == nil:
		task.Attempts[0].Err = errNotFoundJobRun(task.JobRunId)
	case task.jobRun.jobDef == nil:
		task.Attempts[0].Err = errNotFoundJobDef(task.jobRun.JobDefId)
	case task.jobRun.jobDef.handler == nil:
		task.Attempts[0].Err = errNotFoundHandler(task.jobRun.JobDefId, task.jobRun.jobDef.HandlerId)
	case !alreadyCancelled: // now run it
		task.Results, task.Attempts[0].Err = task.jobRun.jobDef.handler.TaskResults(task.jobRun.ctx(ctx, task.Id), task.Details)
		if task.Attempts[0].Err == nil {
			_, task.Attempts[0].Err = handler(task.jobRun.jobDef.HandlerId).wellTypedTaskResults(task.Results)
		}
	}

	task.State, task.FinishTime =
		If(alreadyCancelled, Cancelled, Done), timeNow()
	ctxErr := ctx.Err()
	if ctxErr != nil && errors.Is(ctxErr, context.Canceled) {
		task.State = Cancelled
	} else if (!alreadyCancelled) &&
		((ctxErr != nil && errors.Is(ctxErr, context.DeadlineExceeded)) ||
			((task.Attempts[0].Err != nil) && (task.jobRun.jobDef.handler != nil) &&
				task.jobRun.jobDef.handler.IsTaskErrRetryable(task.Attempts[0].Err))) {
		_ = task.markForRetryOrAsFailed(task.jobRun.jobDef)
	}
	if task.Attempts[0].Err == nil {
		task.Attempts[0].Err = ctxErr
	}
	if _ = it.logErr(log, task.Attempts[0].Err, task); it.logLifecycleEvents(nil, nil, task) {
		task.logger(log).Infof("marking just-%s %s task '%s' (of '%s' job '%s') as %s", If(task.Attempts[0].Err != nil, "failed", "finished"), Running, task.Id, taskJobDefOrType, task.JobRunId, task.State)
	}
	err := it.logErr(log, it.backend.saveJobTask(ctxOrig, task), task)
	if err == nil && it.eventHandlers.onJobTaskExecuted != nil { // only count tasks that actually ran (failed or not) AND were stored
		it.eventHandlers.onJobTaskExecuted(task, timeNow().Sub(*timeStarted))
	}
	return err
}

func (it *engine) manualJobsPossible(ctx context.Context) bool {
	log := loggerNew()
	jobDefManual, err := it.backend.findJobDef(ctx,
		JobDefFilter{}.WithAllowManualJobRuns(true))
	return it.logErr(log, err) != nil || jobDefManual != nil
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
