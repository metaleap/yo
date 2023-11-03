package yojobs

import (
	"context"
	"errors"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	. "yo/util"
	"yo/util/sl"
)

func (me *engine) startDueJob(jobRun *JobRun, jobDef *JobDef) {
	defer Finally(nil)

	if jobDef == nil {
		panic(errNotFoundJobDef(jobRun.Id))
	} else if jobDef.jobType == nil {
		errNotFoundJobType(jobDef.Name, jobDef.JobTypeId)
	}

	time_started := time.Now()
	ctx := NewCtxNonHttp(jobRun.TimeoutPrepAndFinalize(nil), false, "")
	ctx.DbTx()
	ctx_job := jobRun.ctx(ctx, 0)

	// 1. JobType.JobDetails
	var err error
	jobRun.Details, err = jobDef.jobType.JobDetails(ctx_job)
	if err != nil {
		panic(err)
	}
	ctx_job.JobDetails = jobRun.Details

	// 2. JobType.TaskDetails
	task_details_stream := make(chan []TaskDetails)
	go func() {
		defer close(task_details_stream)
		jobDef.jobType.TaskDetails(ctx_job, task_details_stream, func(e error) error {
			if (e != nil) && (err == nil) {
				err = e
			}
			return err
		})
	}()
	var num_tasks int
	for task_details := range task_details_stream {
		if err != nil { // don't `break` here: we need to drain the chan to close it, in the case of...
			continue // ...undisciplined `JobType.TaskDetails` impls (they should stop sending on err)
		}
		tasks := sl.To(task_details, func(it TaskDetails) *JobTask {
			if num_tasks++; err == nil {
				jobType(string(jobRun.JobTypeId)).checkTypeTaskDetails(task_details)
			}
			task := &JobTask{
				JobTypeId: jobRun.JobTypeId,
				state:     yodb.Text(Pending),
				Details:   task_details,
			}
			task.JobRun.SetId(jobRun.Id)
			return task
		})
		yodb.CreateMany[JobTask](ctx, tasks...)
	}
	if err != nil {
		panic(err)
	}

	// 3. update job
	jobRun.state, jobRun.StartTime, jobRun.DurationPrepSecs =
		yodb.Text(Running), yodb.DtNow(), yodb.F32(time.Since(time_started).Seconds())
	yodb.Update[JobRun](ctx, jobRun, nil, false, JobRunFields(jobRunState, JobRunStartTime, JobRunDurationPrepSecs)...)
}

func (me *engine) ensureJobRunSchedules() {
	defer Finally(func() {
		DoAfter(me.options.IntervalEnsureJobSchedules, me.ensureJobRunSchedules)
	})

	ctx := NewCtxNonHttp(TimeoutLong, false, "")
	yodb.Each[JobDef](ctx, q.Not(q.ArrIsEmpty(JobDefSchedules)), 0, nil,
		func(jobDef *JobDef, enough *bool) {
			latest := yodb.FindOne[JobRun](ctx, JobRunJobDef.Equal(jobDef.Id).And(JobRunAutoScheduled.Equal(true)), JobRunDueTime.Desc())
			if (latest != nil) && ((latest.State() == Running) || (latest.State() == JobRunCancelling)) {
				return // still busy: then need no scheduling here & now
			}
			if (latest == nil) || (latest.State() != Pending) { // `latest` is Done or Cancelled (or none)...
				_ = me.scheduleJobRun(ctx, jobDef, latest) // ...so schedule the next
				return
			}

			if latest.DueTime.Time().After(time.Now()) { // `latest` is definitely `Pending` at this point
				var after *yodb.DateTime
				// check-and-maybe-fix the existing Pending job's future DueTime wrt the current `jobDef.Schedules` in case the latter changed after the former was scheduled
				last_done := yodb.FindOne[JobRun](ctx, JobRunJobDef.Equal(jobDef.Id).And(jobRunState.In(Done, Cancelled)), JobRunDueTime.Desc())
				if last_done != nil {
					after = sl.FirstNonNil(last_done.FinishTime, last_done.StartTime, last_done.DueTime)
				}
				due_time := jobDef.findClosestToNowSchedulableTimeSince(after.Time(), true)
				if due_time == nil { // jobDef or all its Schedules were Disabled after that Pending job was scheduled
					me.cancelJobRuns(ctx, map[CancellationReason][]*JobRun{CancellationReasonJobDefChanged: {latest}})
				} else if (!jobDef.ok(*latest.DueTime.Time())) || !due_time.Equal(*latest.DueTime.Time()) {
					// update outdated-by-now DueTime
					latest.DueTime = (*yodb.DateTime)(due_time)
					yodb.Update[JobRun](ctx, latest, nil, false, JobRunFields(JobRunDueTime)...)
				}
			}
		})
}

func (me *engine) scheduleJobRun(ctx *Ctx, jobDef *JobDef, jobRunPrev *JobRun) *JobRun {
	defer Finally(nil)
	if jobDef.Disabled || (jobDef.jobType == nil) {
		return nil
	}
	var last_time *yodb.DateTime
	if jobRunPrev != nil {
		last_time = sl.FirstNonNil(jobRunPrev.FinishTime, jobRunPrev.StartTime, jobRunPrev.DueTime)
	}
	if due_time := jobDef.findClosestToNowSchedulableTimeSince(last_time.Time(), true); due_time != nil {
		return me.createJobRun(ctx, jobDef, yodb.DtFrom(*due_time), nil, jobRunPrev)
	}
	return nil
}

func (me *engine) deleteStorageExpiredJobRuns() {
	defer Finally(func() {
		DoAfter(me.options.IntervalDeleteStorageExpiredJobs, me.deleteStorageExpiredJobRuns)
	})

	ctx := NewCtxNonHttp(TimeoutLong, false, "")
	yodb.Each[JobDef](ctx, JobDefDeleteAfterDays.GreaterThan(0), 0, nil, func(jobDef *JobDef, enough *bool) {
		yodb.Delete[JobRun](ctx, JobRunJobDef.Equal(jobDef.Id).
			And(jobRunState.In(Done, Cancelled)).
			And(JobRunFinishTime.LessThan(time.Now().AddDate(0, 0, -int(jobDef.DeleteAfterDays)))),
		)
	})
}

// A died task is one whose runner died between its start and its finishing or orderly timeout.
// It's found in the DB as still RUNNING despite its timeout moment being over a minute ago:
func (me *engine) expireOrRetryDeadJobTasks() {
	defer Finally(func() {
		DoAfter(me.options.IntervalExpireOrRetryDeadTasks, me.expireOrRetryDeadJobTasks)
	})

	ctx := NewCtxNonHttp(TimeoutLong, false, "")
	jobs := map[yodb.I64][]*JobRun{} // gather candidate jobs for task selection
	{
		for _, job := range yodb.FindMany[JobRun](ctx, jobRunState.Equal(Running), 0, JobRunFields(JobRunId, JobRunJobDef, jobRunState)) {
			jobs[job.JobDef.Id()] = append(jobs[job.JobDef.Id()], job)
		}
	}

	GoItems(sl.Keys(jobs), func(jobDefId yodb.I64) {
		if job_runs := jobs[jobDefId]; len(job_runs) > 0 {
			job_def := job_runs[0].jobDef(ctx)
			for _, job_run := range job_runs[1:] {
				job_run.JobDef = job_runs[0].JobDef // copies the same `self` pointer
			}
			if currently_running := sl.Where(job_runs, func(it *JobRun) bool { return it.State() == Running }); len(currently_running) > 0 {
				me.expireOrRetryDeadJobTasksForJobDef(job_def, sl.To(currently_running, func(it *JobRun) yodb.I64 { return it.Id }))
			}
			if is_jobdef_dead := (job_def == nil) || (job_def.jobType == nil) || job_def.Disabled; is_jobdef_dead {
				for _, job_run := range job_runs {
					job_run.state = yodb.Text(JobRunCancelling)
					yodb.Update[JobRun](ctx, job_run, nil, false, jobRunState.F())
				}
			}
		}
	}, me.options.MaxConcurrentOps)
}

func (me *engine) expireOrRetryDeadJobTasksForJobDef(jobDef *JobDef, runningJobIds []yodb.I64) {
	defer Finally(nil)
	is_jobdef_dead := (jobDef.jobType == nil) || jobDef.Disabled
	ctx := NewCtxNonHttp(TimeoutLong, false, "")
	query := JobTaskJobRun.In(yodb.Arr[yodb.I64](runningJobIds).ToAnys()...)
	if is_jobdef_dead { //  the rare edge case: un-Done/un-Cancelled tasks still in DB for old now-disabled-or-deleted-from-config job def
		query = query.And(jobTaskState.In(Running, Pending))
	} else { // the usual case.
		query = query.And(jobTaskState.Equal(Running)).And(
			JobTaskStartTime.LessThan(time.Now().Add(-(time.Minute + (time.Second * time.Duration(jobDef.TimeoutSecsTaskRun))))))
	}
	yodb.Each[JobTask](ctx, query, 0, nil, func(task *JobTask, enough *bool) {
		defer func() { _ = recover() }()
		task_upd_fields := JobTaskFields(jobTaskState, JobTaskAttempts, JobTaskStartTime, JobTaskFinishTime)
		if is_jobdef_dead {
			task.state = yodb.Text(Cancelled)
			if (len(task.Attempts) > 0) && (task.Attempts[0].Err == nil) {
				task.Attempts[0].Err = context.Canceled
			}
		} else if (!task.markForRetryOrAsFailed(ctx)) && (len(task.Attempts) > 0) && (task.Attempts[0].Err == nil) {
			task.Attempts[0].Err = context.DeadlineExceeded
		}
		yodb.Update[JobTask](ctx, task, qById(task.Id), false, task_upd_fields...)
	})
}

func (me *engine) runJobTasks() {
	defer Finally(func() {
		DoAfter(me.options.IntervalRunTasks, me.runJobTasks)
	})

	ctx := NewCtxNonHttp(TimeoutLong, false, "")
	pending_tasks := yodb.FindMany[JobTask](ctx, jobTaskState.Equal(string(Pending)), me.options.FetchTasksToRun, nil)
	GoItems(pending_tasks, func(it *JobTask) {
		_ = it.jobDef(ctx) // preloads both jobRun and jobDef
	}, me.options.MaxConcurrentOps)

	// ...then run them
	GoItems(pending_tasks, me.runTask, me.options.MaxConcurrentOps)
}

func (me *engine) runTask(task *JobTask) {
	defer Finally(func() {
		if old_canceler := me.setTaskCanceler(task.Id, nil); old_canceler != nil {
			old_canceler()
		} // else: already cancelled by concurrent `finalizeCancelingJobs` call
	})

	time_started := time.Now()
	job_run := task.JobRun.Get(nil) // already preloaded by runJobTasks
	job_def := job_run.jobDef(nil)  // dito
	ctx := NewCtxNonHttp(task.TimeoutRun(nil /*dito*/), true, "")
	if old_cancel := me.setTaskCanceler(task.Id, ctx.Cancel); old_cancel != nil {
		old_cancel() // should never be the case, but let's be principled & clean...
	}
	ctx.DbTx()
	// first, attempt to reserve task for running vs. other pods
	already_canceled := (job_run == nil) || (job_run.State() == Cancelled) || (job_run.State() == JobRunCancelling) ||
		(job_def == nil) || bool(job_def.Disabled) || (job_def.jobType == nil) ||
		(task.JobTypeId != job_def.JobTypeId) || (job_run.JobTypeId != job_def.JobTypeId)

	task_upd_fields := JobTaskFields(jobTaskState, JobTaskFinishTime, JobTaskAttempts)
	task.state, task.FinishTime, task.Attempts =
		yodb.Text(If(already_canceled, Cancelled, Running)), nil, append([]TaskAttempt{taskAttempt()}, task.Attempts...)
	if task.StartTime == nil {
		task.StartTime, task_upd_fields = (*yodb.DateTime)(task.Attempts[0].t), sl.With(task_upd_fields, JobTaskStartTime.F())
	}
	if yodb.Update[JobTask](ctx, task, qById(task.Id), false, task_upd_fields...) <= 0 {
		return // concurrently changed by other instance, possibly in the same run attempt: bug out
	}

	switch {
	case job_run == nil:
		task.Attempts[0].Err = errNotFoundJobRun(task.JobRun.Id())
	case job_def == nil:
		task.Attempts[0].Err = errNotFoundJobDef(job_run.Id)
	case job_def.jobType == nil:
		task.Attempts[0].Err = errNotFoundJobType(job_def.Name, job_def.JobTypeId)
	case !already_canceled: // actual RUNNING of task
		results, err := job_def.jobType.TaskResults(job_run.ctx(ctx, task.Id), task.Details)
		if task.Results = results; err != nil {
			task.Attempts[0].Err = err
		}
		if task.Attempts[0].Err == nil {
			jobType(string(job_def.JobTypeId)).checkTypeJobResults(task.Results)
			task_upd_fields = sl.With(task_upd_fields, jobTaskResults.F())
		}
	}

	task.state, task.FinishTime =
		yodb.Text(If(already_canceled, Cancelled, Done)), yodb.DtNow()
	err_ctx := ctx.Err()
	did_mark_for_retry := false
	if err_ctx != nil && errors.Is(err_ctx, context.Canceled) {
		task.state = yodb.Text(Cancelled)
	} else if (!already_canceled) &&
		(((err_ctx != nil) && errors.Is(err_ctx, context.DeadlineExceeded)) ||
			((task.Attempts[0].Err != nil) && (job_def.jobType != nil) &&
				job_def.jobType.IsTaskErrRetryable(task.Attempts[0].Err))) {
		did_mark_for_retry = task.markForRetryOrAsFailed(nil)
	}
	if task.Attempts[0].Err == nil {
		task.Attempts[0].Err = err_ctx
	}
	if (task.Attempts[0].Err != nil) && did_mark_for_retry {
		task_upd_fields = sl.With(task_upd_fields, JobTaskStartTime.F())
	}

	// ready to save
	task_upd_fields = sl.Without(task_upd_fields, JobTaskStartTime.F())
	did_store := (0 < yodb.Update[JobTask](ctx, task, qById(task.Id), false, task_upd_fields...))
	if did_store && (me.eventHandlers.onJobTaskExecuted != nil) {
		me.eventHandlers.onJobTaskExecuted(task, time.Now().Sub(time_started))
	}
}
