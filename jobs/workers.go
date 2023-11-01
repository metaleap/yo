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

	Timeout(ctxNone, TimeoutLong, it.finalizeJobRunsIfDone)
	Timeout(ctxNone, TimeoutLong, it.finalizeCancelingJobRuns)
	Timeout(ctxNone, TimeoutLong, it.startDueJobRuns)
}

func (it *engine) finalizeJobRunsIfDone(ctx context.Context) {
	log := loggerNew()
	var running_jobs []*JobRun
	Timeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.storage.listJobRuns(ctx, true, false, noPaging,
			JobRunFilter{}.WithStates(Running))
		if nil == it.logErr(log, err) {
			running_jobs = jobs
		}
	})

	cancel_jobs := map[CancellationReason][]*JobRun{}
	for cancel_reason, check := range map[CancellationReason]func(*JobRun) bool{
		CancellationReasonDefInvalidOrGone: func(j *JobRun) bool {
			return (j.jobDef == nil)
		},
		CancellationReasonDefChanged: func(j *JobRun) bool {
			return (j.jobDef != nil) && ((j.JobTypeId != j.jobDef.JobTypeId) || j.jobDef.Disabled)
		},
		CancellationReasonJobTypeInvalidOrGone: func(j *JobRun) bool {
			return (j.jobDef != nil) && (j.jobDef.JobTypeId == j.JobTypeId) && (j.jobDef.jobType == nil)
		},
	} {
		jobs_to_cancel_due_to_check := sl.Where(running_jobs, check)
		cancel_jobs[cancel_reason], running_jobs =
			jobs_to_cancel_due_to_check, sl.Without(running_jobs, jobs_to_cancel_due_to_check...)
	}
	for job, err := range it.cancelJobRuns(ctx, cancel_jobs) {
		_ = it.logErr(log, err, job)
	}
	GoItems(ctx, running_jobs, it.finalizeJobRunIfDone,
		it.options.MaxConcurrentOps, 0 /* hence, JobRun.Timeout() */)
}

func (it *engine) finalizeJobRunIfDone(ctx context.Context, jobRun *JobRun) {
	log := loggerNew()
	still_busy, err := it.storage.findJobTask(ctx, false, false,
		JobTaskFilter{}.WithStates(Pending, Running).WithJobRuns(jobRun.Id))
	if (nil != it.logErr(log, err, jobRun)) || (still_busy != nil) {
		return
	}

	tasks_filter, tasks_list_req, time_started :=
		JobTaskFilter{}.WithJobRuns(jobRun.Id), ListRequest{}, timeNow()
	if jobRun.FinalTaskFilter != nil {
		tasks_filter = jobRun.FinalTaskFilter.WithJobRuns(jobRun.Id)
	}
	if jobRun.FinalTaskListReq != nil {
		tasks_list_req = *jobRun.FinalTaskListReq
	}
	var tasks_stream chan *JobTask
	abort_streaming := false
	jobRun.Results, err = jobRun.jobDef.jobType.JobResults(jobRun.ctx(ctx, ""), func() <-chan *JobTask {
		if tasks_stream == nil {
			tasks_stream = make(chan *JobTask, Clamp(0, 1024, tasks_list_req.PageSize))
			go func(ctx context.Context) {
				defer close(tasks_stream)
				for tasks_list_req.PageToken = ""; !abort_streaming; { // bools dont need a mutex =)
					tasksPage, _, _, nextPageTok, err := it.storage.listJobTasks(ctx, false, false,
						tasks_list_req, tasks_filter)
					if nil != it.logErr(log, err, jobRun) {
						return
					}
					for _, task := range tasksPage {
						if tasks_stream <- task; abort_streaming {
							break
						}
					}
					if tasks_list_req.PageToken = nextPageTok; tasks_list_req.PageToken == "" {
						break
					}
				}
			}(ctx)
		}
		return tasks_stream
	})
	abort_streaming = true
	if err == nil {
		_, err = jobType(jobRun.jobDef.JobTypeId).wellTypedJobResults(jobRun.Results)
	}
	if nil != it.logErr(log, err, jobRun) {
		return
	}
	time_now := timeNow()
	jobRun.State, jobRun.FinishTime, jobRun.Info.DurationFinalizeSecs =
		Done, time_now, ToPtr(time_now.Sub(*time_started).Seconds())

	if it.logLifecycleEvents(nil, jobRun, nil) {
		jobRun.logger(log).Infof("marking %s '%s' job run '%s' as %s", Running, jobRun.JobDefId, jobRun.Id, Done)
	}
	if (nil == it.logErr(log, it.storage.transacted(ctx, func(ctx context.Context) error {
		err = it.storage.saveJobRun(ctx, jobRun)
		if (err == nil) && jobRun.AutoScheduled {
			// doing this right here upon job finalization (in the same
			// transaction) helps prevent concurrent duplicate job schedulings.
			_ = it.logErr(log, it.scheduleJob(ctx, jobRun.jobDef, jobRun), jobRun)
		}
		return err
	}), jobRun)) && (it.eventHandlers.onJobRunExecuted != nil) { // only count jobs that ran AND were stored
		if job_stats, err := it.stats(ctx, jobRun); err == nil {
			it.eventHandlers.onJobRunExecuted(jobRun, job_stats)
		}
	}
}

func (it *engine) finalizeCancelingJobRuns(ctx context.Context) {
	log := loggerNew()
	var cancel_jobruns []*JobRun
	Timeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		job_runs, _, _, err := it.storage.listJobRuns(ctx, true, false, noPaging,
			JobRunFilter{}.WithStates(JobRunCancelling))
		if it.logErr(log, err) == nil {
			cancel_jobruns = job_runs
		}
	})
	GoItems(ctx, cancel_jobruns, it.finalizeCancelingJobRun,
		it.options.MaxConcurrentOps, TimeoutLong)
}

func (it *engine) finalizeCancelingJobRun(ctx context.Context, job *JobRun) {
	log := loggerNew()
	var num_canceled, num_tasks_to_cancel int

	list_req := ListRequest{PageSize: 444}
	for {
		job_tasks, _, _, page_tok, err := it.storage.listJobTasks(ctx, false, false, list_req,
			JobTaskFilter{}.WithJobRuns(job.Id).WithStates(Pending, Running))
		if it.logErr(log, err, job) != nil {
			return
		}
		list_req.PageToken, num_tasks_to_cancel = page_tok, num_tasks_to_cancel+len(job_tasks)
		for _, job_task := range job_tasks {
			if canceler := it.setTaskCanceler(job_task.Id, nil); canceler != nil {
				go canceler()
			} // this is optional/luxury, but nice if it succeeds due to (by chance) the Task being still Running & on this very same pod.

			job_task.jobRun = job
			state := job_task.State
			job_task.State = Cancelled
			if it.logLifecycleEvents(nil, nil, job_task) {
				job_task.logger(log).Infof("marking %s job task '%s' (of '%s' job '%s') as %s", state, job_task.Id, job_task.JobTypeId, job_task.JobRunId, job_task.State)
			}
			if nil == it.logErr(log, it.storage.saveJobTask(ctx, job_task), job_task) {
				num_canceled++
			}
		}
		if page_tok == "" {
			break
		}
	}
	if num_tasks_to_cancel == num_canceled { // no more tasks left to cancel, now finalize
		job.State, job.FinishTime =
			Cancelled, timeNow()
		if it.logLifecycleEvents(nil, job, nil) {
			job.logger(log).Infof("marking %s '%s' job '%s' as %s", JobRunCancelling, job.JobDefId, job.Id, Cancelled)
		}
		_ = it.logErr(log, it.storage.transacted(ctx, func(ctx context.Context) error {
			err := it.logErr(log, it.storage.saveJobRun(ctx, job), job)
			if (err == nil) && job.AutoScheduled && (job.jobDef != nil) && !job.jobDef.Disabled {
				// doing this right here upon job finalization (in the same
				// transaction) prevents concurrent duplicate job schedulings.
				err = it.scheduleJob(ctx, job.jobDef, job)
			}
			return err
		}), job)
	}
}

func (it *engine) startDueJobRuns(ctx context.Context) {
	log := loggerNew()
	var due_jobs []*JobRun
	Timeout(ctx, it.options.TimeoutShort, func(ctx context.Context) {
		jobs, _, _, err := it.storage.listJobRuns(ctx, true, false, noPaging,
			JobRunFilter{}.WithStates(Pending).WithDue(true))
		if it.logErr(log, err) == nil {
			due_jobs = jobs
		}
	})
	{ // cancel rare duplicates and remnants of by-now-removed/disabled job-defs
		cancelJobs := map[CancellationReason][]*JobRun{}
		sort.Slice(due_jobs, func(i int, j int) bool { return !due_jobs[i].AutoScheduled })
		for i := 0; i < len(due_jobs); i++ {
			job := due_jobs[i]
			idx := sl.IdxWhere(due_jobs, func(j *JobRun) bool { // check if duplicate
				return j.JobDefId == job.JobDefId && cmp.Equal(j.Details, job.Details, cmpopts.IgnoreUnexported(), cmpopts.EquateEmpty())
			})
			var reason CancellationReason
			switch {
			case idx != i:
				reason = CancellationReasonDuplicate
			case job.jobDef == nil:
				reason = CancellationReasonDefInvalidOrGone
			case job.JobTypeId != job.jobDef.JobTypeId || job.jobDef.Disabled:
				reason = CancellationReasonDefChanged
			case job.jobDef.jobType == nil && job.JobTypeId == job.jobDef.JobTypeId:
				reason = CancellationReasonJobTypeInvalidOrGone
			}
			if reason != "" {
				cancelJobs[reason] = append(cancelJobs[reason], job)
				due_jobs = append(due_jobs[:i], due_jobs[i+1:]...)
				i--
			}
		}
		for job, err := range it.cancelJobRuns(ctx, cancelJobs) {
			_ = it.logErr(log, err, job)
		}
	}
	GoItems(ctx, due_jobs, it.startDueJob,
		it.options.MaxConcurrentOps, 0 /* thus uses Job.Timeout() */)
}

func (it *engine) startDueJob(ctx context.Context, job *JobRun) {
	log := loggerNew()
	var err error
	if job.jobDef == nil {
		err = errNotFoundJobDef(job.JobDefId)
	} else if job.jobDef.jobType == nil {
		err = errNotFoundJobType(job.jobDef.Id, job.jobDef.JobTypeId)
	}
	if it.logErr(log, err, job) != nil {
		return
	}

	// 1. JobType.JobDetails
	timeStarted, jobCtx := timeNow(), job.ctx(ctx, "")
	if job.Details == nil {
		if job.Details, err = job.jobDef.defaultJobDetails(); it.logErr(log, err, job) != nil {
			return
		}
	}
	if job.Details, err = job.jobDef.jobType.JobDetails(jobCtx); it.logErr(log, err, job) != nil {
		return
	}
	if _, err = jobType(job.jobDef.JobTypeId).wellTypedJobDetails(job.Details); it.logErr(log, err, job) != nil {
		return
	}

	// 2. JobType.TaskDetails
	taskDetailsStream := make(chan []TaskDetails)
	go func() {
		defer close(taskDetailsStream)
		job.FinalTaskListReq, job.FinalTaskFilter = job.jobDef.jobType.TaskDetails(jobCtx, taskDetailsStream, func(e error) error {
			if e != nil && err == nil {
				err = e
			}
			return err
		})
	}()
	_ = it.logErr(log, it.storage.transacted(ctx, func(ctx context.Context) error {
		var numTasks int
		for taskDetails := range taskDetailsStream {
			if err != nil { // don't `break` here: we need to drain the chan to close it, in the case of...
				continue // ...undisciplined `JobType.TaskDetails` impls (they should stop sending on err)
			}
			tasks := sl.To(taskDetails, func(details TaskDetails) *JobTask {
				if numTasks++; err == nil {
					_, err = jobType(job.jobDef.JobTypeId).wellTypedTaskDetails(details)
				}
				return &JobTask{
					Id:         job.Id + "_" + strconv.Itoa(numTasks),
					JobRunId:   job.Id,
					JobTypeId:  job.JobTypeId,
					State:      Pending,
					FinishTime: nil,
					StartTime:  nil,
					Version:    1,
					Details:    details,
				}
			})
			if len(tasks) > 0 && err == nil {
				err = it.storage.insertJobTasks(ctx, tasks...)
			}
		}
		if err == nil {
			job.State, job.StartTime, job.FinishTime, job.Info.DurationPrepSecs =
				Running, timeNow(), nil, ToPtr(timeNow().Sub(*timeStarted).Seconds())
			if it.logLifecycleEvents(nil, job, nil) {
				job.logger(log).Infof("marking %s '%s' job '%s' as %s (with %d tasks)", Pending, job.JobDefId, job.Id, job.State, numTasks)
			}
			err = it.storage.saveJobRun(ctx, job)
		}
		return err
	}), job)
}

func (it *engine) ensureJobRunSchedules() {
	defer doAfter(it.options.IntervalEnsureJobSchedules, it.ensureJobRunSchedules)

	var jobDefs []*JobDef
	{
		var err error
		log := loggerNew()
		jobDefs, err = it.storage.listJobDefs(ctxNone,
			JobDefFilter{}.WithDisabled(false).WithEnabledSchedules())
		it.logErr(log, err)
	}
	GoItems(ctxNone, jobDefs, it.ensureJobDefScheduled,
		it.options.MaxConcurrentOps, it.options.TimeoutShort)
}

func (it *engine) ensureJobDefScheduled(ctx context.Context, jobDef *JobDef) {
	log := loggerNew()

	latest, err := it.storage.findJobRun(ctx, false, false, // defaults to sorted descending by due_time
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
		lastDone, err := it.storage.findJobRun(ctx, false, false,
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
			_ = it.logErr(log, it.storage.saveJobRun(ctx, latest), latest)
		}
	}
}

func (it *engine) scheduleJob(ctx context.Context, jobDef *JobDef, last *JobRun) error {
	if jobDef.Disabled || jobDef.jobType == nil {
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

	Timeout(ctxNone, it.options.TimeoutShort, func(ctx context.Context) {
		log := loggerNew()
		jobDefs, err := it.storage.listJobDefs(ctx, JobDefFilter{}.WithStorageExpiry(true))
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
	jobsToDelete, _, _, err := it.storage.listJobRuns(ctx, true, false, noPaging,
		JobRunFilter{}.WithStates(Done, Cancelled).WithJobDefs(jobDef.Id).
			WithFinishedBefore(timeNow().AddDate(0, 0, -jobDef.DeleteAfterDays)))
	if it.logErr(log, err, jobDef) != nil {
		return
	}

	for _, job := range jobsToDelete {
		if it.logLifecycleEvents(nil, job, nil) {
			job.logger(log).Infof("deleting %s '%s' job '%s' and its tasks", job.State, jobDef.Id, job.Id)
		}
		_ = it.logErr(log, it.storage.transacted(ctx, func(ctx context.Context) error {
			err := it.storage.deleteJobTasks(ctx, JobTaskFilter{}.WithJobRuns(job.Id))
			if err == nil {
				err = it.storage.deleteJobRuns(ctx, JobRunFilter{}.WithIds(job.Id))
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
		jobs, _, _, err := it.storage.listJobRuns(ctxNone, true, false, noPaging,
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
	defDead, taskFilter := jobDef == nil || jobDef.Disabled || jobDef.jobType == nil, JobTaskFilter{}.
		WithJobRuns(sl.To(runningJobs, func(v *JobRun) string { return v.Id })...)
	if !defDead { // the usual case.
		taskFilter = taskFilter.WithStates(Running).WithStartedBefore(timeNow().Add(-(jobDef.Timeouts.TaskRun + time.Minute)))
	} else { //  the rare edge case: un-Done tasks still in DB for old now-disabled-or-deleted-from-config job def
		taskFilter = taskFilter.WithStates(Running, Pending)
	}
	deadTasks, _, _, _, err := it.storage.listJobTasks(ctx, false, false, noPaging, taskFilter)
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
		_ = it.logErr(log, it.storage.saveJobTask(ctx, task), task)
	}
}

func (it *engine) runJobTasks() {
	defer func() { doAfter(it.options.IntervalRunTasks, it.runJobTasks) }()

	var pendingTasks []*JobTask
	{
		var err error
		log := loggerNew()
		pendingTasks, _, _, _, err = it.storage.listJobTasks(ctxNone, true, false, ListRequest{PageSize: it.options.FetchTasksToRun},
			JobTaskFilter{}.WithStates(Pending))
		if it.logErr(log, err) != nil {
			return
		}
	}

	// ...then run them
	GoItems(ctxNone, pendingTasks, func(ctx context.Context, task *JobTask) {
		_ = it.runTask(ctx, task)
	}, it.options.MaxConcurrentOps, 0 /* hence, task.Timeout() */)
}

func (it *engine) runTask(ctx context.Context, task *JobTask) error {
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

	taskJobDefOrType := task.JobTypeId
	if task.jobRun != nil {
		taskJobDefOrType = task.jobRun.JobDefId
	}
	// first, attempt to reserve task for running vs. other pods
	alreadyCancelled := task.jobRun == nil || task.jobRun.State == Cancelled || task.jobRun.State == JobRunCancelling ||
		task.jobRun.jobDef == nil || task.jobRun.jobDef.Disabled || task.jobRun.jobDef.jobType == nil ||
		task.JobTypeId != task.jobRun.jobDef.JobTypeId || task.jobRun.JobTypeId != task.jobRun.jobDef.JobTypeId
	oldTaskState := task.State
	task.State, task.FinishTime, task.Attempts =
		If(alreadyCancelled, Cancelled, Running), nil, append([]*TaskAttempt{{Time: *timeNow()}}, task.Attempts...)
	if task.StartTime == nil {
		task.StartTime = timeNow()
	}
	if it.logLifecycleEvents(nil, nil, task) {
		task.logger(log).Infof("marking %s task '%s' (of '%s' job '%s') as %s", oldTaskState, task.Id, taskJobDefOrType, task.JobRunId, task.State)
	}
	if err := it.logErr(log, it.storage.saveJobTask(ctx, task), task); err != nil {
		return err
	}

	switch {
	case task.jobRun == nil:
		task.Attempts[0].Err = errNotFoundJobRun(task.JobRunId)
	case task.jobRun.jobDef == nil:
		task.Attempts[0].Err = errNotFoundJobDef(task.jobRun.JobDefId)
	case task.jobRun.jobDef.jobType == nil:
		task.Attempts[0].Err = errNotFoundJobType(task.jobRun.JobDefId, task.jobRun.jobDef.JobTypeId)
	case !alreadyCancelled: // now run it
		task.Results, task.Attempts[0].Err = task.jobRun.jobDef.jobType.TaskResults(task.jobRun.ctx(ctx, task.Id), task.Details)
		if task.Attempts[0].Err == nil {
			_, task.Attempts[0].Err = jobType(task.jobRun.jobDef.JobTypeId).wellTypedTaskResults(task.Results)
		}
	}

	task.State, task.FinishTime =
		If(alreadyCancelled, Cancelled, Done), timeNow()
	ctxErr := ctx.Err()
	if ctxErr != nil && errors.Is(ctxErr, context.Canceled) {
		task.State = Cancelled
	} else if (!alreadyCancelled) &&
		((ctxErr != nil && errors.Is(ctxErr, context.DeadlineExceeded)) ||
			((task.Attempts[0].Err != nil) && (task.jobRun.jobDef.jobType != nil) &&
				task.jobRun.jobDef.jobType.IsTaskErrRetryable(task.Attempts[0].Err))) {
		_ = task.markForRetryOrAsFailed(task.jobRun.jobDef)
	}
	if task.Attempts[0].Err == nil {
		task.Attempts[0].Err = ctxErr
	}
	if _ = it.logErr(log, task.Attempts[0].Err, task); it.logLifecycleEvents(nil, nil, task) {
		task.logger(log).Infof("marking just-%s %s task '%s' (of '%s' job '%s') as %s", If(task.Attempts[0].Err != nil, "failed", "finished"), Running, task.Id, taskJobDefOrType, task.JobRunId, task.State)
	}
	err := it.logErr(log, it.storage.saveJobTask(ctxOrig, task), task)
	if err == nil && it.eventHandlers.onJobTaskExecuted != nil { // only count tasks that actually ran (failed or not) AND were stored
		it.eventHandlers.onJobTaskExecuted(task, timeNow().Sub(*timeStarted))
	}
	return err
}

func (it *engine) manualJobsPossible(ctx context.Context) bool {
	log := loggerNew()
	jobDefManual, err := it.storage.findJobDef(ctx,
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
