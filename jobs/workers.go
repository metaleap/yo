package yojobs

import (
	"context"
	"errors"
	"time"

	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	. "yo/util"
	"yo/util/kv"
	"yo/util/sl"
	"yo/util/str"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func (me *engine) startAndFinalizeJobRuns() {
	defer DoAfter(me.options.IntervalStartAndFinalizeJobs, me.startAndFinalizeJobRuns)

	me.finalizeDoneJobRuns()
	me.startDueJobRuns()
	me.finalizeCancellingJobRuns()
}

func (me *engine) finalizeDoneJobRuns() {
	ctx := NewCtxNonHttp(Timeout1Min, false, "")
	defer ctx.OnDone(nil)
	job_runs := yodb.FindMany[JobRun](ctx, jobRunState.Equal(Running), 0, nil)

	cancel_jobs := map[CancellationReason][]*JobRun{}
	for reason, check := range map[CancellationReason]func(*JobRun) bool{
		CancellationReasonJobDefInvalidOrGone: func(it *JobRun) bool {
			return (it.jobDef(ctx) == nil)
		},
		CancellationReasonJobDefChanged: func(it *JobRun) bool {
			job_def := it.jobDef(ctx)
			return (job_def != nil) && ((it.JobTypeId != job_def.JobTypeId) || bool(job_def.Disabled))
		},
		CancellationReasonJobTypeInvalidOrGone: func(it *JobRun) bool {
			job_def := it.jobDef(ctx)
			return (job_def != nil) && (job_def.JobTypeId == it.JobTypeId) && (job_def.jobType == nil)
		},
	} {
		jobs_to_cancel_due_to_check := sl.Where(job_runs, check)
		cancel_jobs[reason], job_runs =
			jobs_to_cancel_due_to_check, sl.Without(job_runs, jobs_to_cancel_due_to_check...)
	}

	GoItems(job_runs, func(it *JobRun) {
		me.finalizeDoneJobRun(ctx, it)
	}, me.options.MaxConcurrentOps)

	me.cancelJobRuns(ctx, cancel_jobs)
}

func (me *engine) finalizeDoneJobRun(ctxForCacheReuse *Ctx, jobRun *JobRun) {
	job_def, time_started := jobRun.jobDef(ctxForCacheReuse), time.Now()
	ctx := ctxForCacheReuse.CopyButWith(jobRun.TimeoutPrepAndFinalize(ctxForCacheReuse), false)
	defer ctx.OnDone(nil)

	if yodb.Exists[JobTask](ctx, JobTaskJobRun.Equal(jobRun.id()).And(jobTaskState.In(Pending, Running))) {
		return // still have busy tasks in job, it's not Done
	}
	if (job_def == nil) || (job_def.jobType == nil) || job_def.Disabled {
		jobRun.state = yodb.Text(JobRunCancelling)
		yodb.Update[JobRun](ctx, jobRun, nil, false, JobRunFields(jobRunState)...)
		return
	}

	on_next_task, final_results := job_def.jobType.JobResults(jobRun.ctx(ctx, 0))
	if on_next_task != nil {
		var ctx_sep *Ctx
		yodb.Each[JobTask](ctx, JobTaskJobRun.Equal(jobRun.Id), 0, nil, func(rec *JobTask, enough *bool) {
			on_next_task(func() *Ctx {
				if ctx_sep == nil {
					ctx_sep = ctx.CopyButWith(-1, false)
				}
				return ctx_sep
			}, rec, enough)
		})
		ctx_sep.OnDone(nil)
	}
	if final_results != nil {
		jobRun.Results = final_results()
	}

	ctx.DbTx(false)
	jobType(string(job_def.JobTypeId)).checkTypeJobResults(jobRun.Results)
	jobRun.state, jobRun.FinishTime, jobRun.DurationFinalizeSecs =
		yodb.Text(Done), yodb.DtNow(), yodb.F32(time.Since(time_started).Seconds())
	yodb.Update[JobRun](ctx, jobRun, nil, false, JobRunFields(jobRunState, jobRunResults, JobRunFinishTime, JobRunDurationFinalizeSecs)...)
	me.scheduleJobRun(ctx, job_def, jobRun)
}

func (me *engine) finalizeCancellingJobRuns() {
	ctx := NewCtxNonHttp(Timeout1Min, false, "")
	defer ctx.OnDone(nil)

	// no db-tx(s) wanted here by design, as many task cancellations as we can get are fine for us, running on an interval anyway
	jobs := yodb.FindMany[JobRun](ctx, jobRunState.Equal(JobRunCancelling), 0, JobRunFields(JobRunId, JobRunVersion))
	if len(jobs) > 0 {
		tasks := yodb.FindMany[JobTask](ctx, jobTaskState.In(Pending, Running).And(JobTaskJobRun.In(sl.To(jobs, (*JobRun).id).ToAnys()...)), 0, JobTaskFields(JobTaskId, JobTaskVersion))
		dbBatchUpdate[JobTask](me, ctx, tasks, &JobTask{state: yodb.Text(Cancelled)}, JobTaskFields(jobTaskState)...)
		dbBatchUpdate[JobRun](me, ctx, jobs, &JobRun{state: yodb.Text(Cancelled)}, JobRunFields(jobRunState)...)
	}
}

func (me *engine) startDueJobRuns() {
	ctx := NewCtxNonHttp(Timeout1Min, false, "")
	defer ctx.OnDone(nil)

	jobs_due := yodb.FindMany[JobRun](ctx, jobRunState.Equal(Pending).And(JobRunDueTime.LessThan(time.Now())), 0, nil, JobRunDueTime.Asc())
	jobs_cancel := map[CancellationReason][]*JobRun{}
	for i := (len(jobs_due) - 1); i >= 0; i-- {
		this := jobs_due[i]
		idx_dupl := sl.IdxWhere(jobs_due[:i], func(it *JobRun) bool {
			return (it.JobDef.Id() == this.JobDef.Id()) &&
				cmp.Equal(it.Details, this.Details, cmpopts.IgnoreUnexported(), cmpopts.EquateEmpty())
		})
		jobdef := this.jobDef(ctx)
		var reason CancellationReason
		switch {
		case (idx_dupl >= 0):
			reason = CancellationReasonJobRunDuplicate
		case jobdef == nil:
			reason = CancellationReasonJobDefInvalidOrGone
		case (this.JobTypeId != jobdef.JobTypeId) || bool(jobdef.Disabled):
			reason = CancellationReasonJobDefChanged
		case (jobdef.jobType == nil) && (this.JobTypeId == jobdef.JobTypeId):
			reason = CancellationReasonJobTypeInvalidOrGone
		}
		if reason != "" {
			jobs_cancel[reason] = append(jobs_cancel[reason], this)
			jobs_due = append(jobs_due[:i], jobs_due[i+1:]...)
		}
	}

	GoItems(jobs_due, func(it *JobRun) {
		me.startDueJob(ctx, it, it.jobDef(ctx))
	}, me.options.MaxConcurrentOps)

	me.cancelJobRuns(ctx, jobs_cancel)
}

func (me *engine) startDueJob(ctxForCacheReuse *Ctx, jobRun *JobRun, jobDef *JobDef) {
	time_started := time.Now()
	if jobDef == nil {
		panic(errNotFoundJobDef(jobRun.Id))
	} else if jobDef.jobType == nil {
		panic(errNotFoundJobType(jobDef.Name, jobDef.JobTypeId))
	}

	ctx := ctxForCacheReuse.CopyButWith(jobRun.TimeoutPrepAndFinalize(ctxForCacheReuse), false)
	defer ctx.OnDone(nil)
	ctx.DbTx(false)

	// 1. JobType.JobDetails
	jobRun.Details = jobDef.jobType.JobDetails(jobRun.ctx(ctx, 0))
	jobType(string(jobDef.JobTypeId)).checkTypeJobDetails(jobRun.Details)

	// 2. JobType.TaskDetails
	done, num_tasks := false, 0
	jobDef.jobType.TaskDetails(jobRun.ctx(ctx, 0), func(multipleTaskDetails []TaskDetails) {
		if done {
			panic(jobDef.Name + ".TaskDetails: illegal call to feed func after return")
		}
		tasks := sl.To(multipleTaskDetails, func(taskDetails TaskDetails) *JobTask {
			jobType(string(jobRun.JobTypeId)).checkTypeTaskDetails(taskDetails)
			task := &JobTask{
				JobTypeId: jobRun.JobTypeId,
				state:     yodb.Text(Pending),
				Details:   taskDetails,
			}
			task.JobRun.SetId(jobRun.Id)
			return task
		})
		num_tasks += len(tasks)
		ctx.Db.PrintRawSqlInDevMode = true
		yodb.CreateMany[JobTask](ctx, tasks...)
	})
	done = true

	// 3. update job
	if (num_tasks > 0) || jobDef.RunTasklessJobs {
		jobRun.state, jobRun.StartTime, jobRun.DurationPrepSecs =
			yodb.Text(Running), yodb.DtNow(), yodb.F32(time.Since(time_started).Seconds())
		yodb.Update[JobRun](ctx, jobRun, nil, false, JobRunFields(jobRunDetails, jobRunState, JobRunStartTime, JobRunDurationPrepSecs)...)
	} // else: just stays pending. eventually, it produces tasks. if never, it just remains with no need to cancel, re-schedule, cancel, re-schedule, etc.
}

func (me *engine) ensureJobRunSchedules() {
	ctx := NewCtxNonHttp(Timeout1Min, false, "")
	defer ctx.OnDone(func() {
		DoAfter(me.options.IntervalEnsureJobSchedules, me.ensureJobRunSchedules)
	})

	cancel_jobs := map[CancellationReason][]*JobRun{}
	job_defs := yodb.FindMany[JobDef](ctx, q.Not(q.ArrIsEmpty(JobDefSchedules)), 0, nil /* keep it all-fields due to JobDef.OnAfterLoaded */)
	for _, job_def := range job_defs {
		latest := yodb.FindOne[JobRun](ctx, JobRunJobDef.Equal(job_def.Id).And(JobRunAutoScheduled.Equal(true)), JobRunDueTime.Desc())
		if (latest != nil) && ((latest.State() == Running) || (latest.State() == JobRunCancelling)) {
			continue // still busy: then need no scheduling here & now
		}
		if (latest == nil) || (latest.State() != Pending) { // `latest` is Done or Cancelled (or none)...
			_ = me.scheduleJobRun(ctx, job_def, latest) // ...so schedule the next
			continue
		}

		if latest.DueTime.Time().After(time.Now()) { // `latest` is definitely `Pending` at this point
			var after *yodb.DateTime
			// check-and-maybe-fix the existing Pending job's future DueTime wrt the current `jobDef.Schedules` in case the latter changed after the former was scheduled
			last_done := yodb.FindOne[JobRun](ctx, JobRunJobDef.Equal(job_def.Id).And(jobRunState.In(Done, Cancelled)), JobRunDueTime.Desc())
			if last_done != nil {
				after = sl.FirstNonNil(last_done.FinishTime, last_done.StartTime, last_done.DueTime)
			}
			due_time := job_def.findClosestToNowSchedulableTimeSince(after.Time(), true)
			if due_time == nil { // jobDef or all its Schedules were Disabled after that Pending job was scheduled
				cancel_jobs[CancellationReasonJobDefChanged] = append(cancel_jobs[CancellationReasonJobDefChanged], latest)
			} else if (!job_def.ok(*latest.DueTime.Time())) || !due_time.Equal(*latest.DueTime.Time()) {
				// update outdated-by-now DueTime
				latest.DueTime = (*yodb.DateTime)(due_time)
				yodb.Update[JobRun](ctx, latest, nil, false, JobRunFields(JobRunDueTime)...)
			}
		}
	}

	me.cancelJobRuns(ctx, cancel_jobs)
}

func (me *engine) scheduleJobRun(ctx *Ctx, jobDef *JobDef, jobRunPrev *JobRun) *JobRun {
	if jobDef.Disabled || (jobDef.jobType == nil) {
		return nil
	}
	var last_time *yodb.DateTime
	if jobRunPrev != nil {
		last_time = sl.FirstNonNil(jobRunPrev.FinishTime, jobRunPrev.StartTime, jobRunPrev.DueTime)
	}
	due_time := jobDef.findClosestToNowSchedulableTimeSince(last_time.Time(), true)
	if due_time != nil {
		return me.createJobRun(ctx, jobDef, yodb.DtFrom(*due_time), jobRunPrev, true)
	}
	return nil
}

func (me *engine) deleteStorageExpiredJobRuns() {
	ctx := NewCtxNonHttp(Timeout1Min, false, "")
	defer ctx.OnDone(func() {
		DoAfter(me.options.IntervalDeleteStorageExpiredJobs, me.deleteStorageExpiredJobRuns)
	})

	job_defs := yodb.FindMany[JobDef](ctx, JobDefDeleteAfterDays.GreaterThan(0), 0, nil /* keep it all-fields due to JobDef.OnAfterLoaded */)
	for _, job_def := range job_defs {
		yodb.Delete[JobRun](ctx, JobRunJobDef.Equal(job_def.Id).
			And(jobRunState.In(Done, Cancelled)).
			And(JobRunFinishTime.LessThan(time.Now().AddDate(0, 0, -int(job_def.DeleteAfterDays)))),
		)
	}
}

// A died task is one whose runner died between its start and its finishing or orderly timeout.
// It's found in the DB as still RUNNING despite its timeout moment being over a minute ago:
func (me *engine) expireOrRetryDeadJobTasks() {
	ctx := NewCtxNonHttp(Timeout1Min, false, "")
	defer ctx.OnDone(func() {
		DoAfter(me.options.IntervalExpireOrRetryDeadTasks, me.expireOrRetryDeadJobTasks)
	})

	jobs := sl.Grouped(
		yodb.FindMany[JobRun](ctx, jobRunState.Equal(Running), 0, JobRunFields(JobRunId, JobRunJobDef, jobRunState, JobRunVersion)),
		func(it *JobRun) yodb.I64 { return it.JobDef.Id() },
	)

	GoItems(kv.Keys(jobs), func(jobDefId yodb.I64) {
		ctx := ctx.CopyButWith(-1, false)
		defer ctx.OnDone(nil)
		if job_runs := jobs[jobDefId]; len(job_runs) > 0 {
			job_def := job_runs[0].jobDef(ctx)
			me.expireOrRetryDeadJobTasksForJobDef(ctx, job_def, sl.To(job_runs, (*JobRun).id))

			if is_jobdef_dead := (job_def == nil) || (job_def.jobType == nil) || (job_def.Disabled); is_jobdef_dead {
				dbBatchUpdate(me, ctx, job_runs, &JobRun{state: yodb.Text(JobRunCancelling)}, JobRunFields(jobRunState)...)
			}
		}
	}, me.options.MaxConcurrentOps)
}

func (me *engine) expireOrRetryDeadJobTasksForJobDef(ctx *Ctx, jobDef *JobDef, runningJobIds sl.Of[yodb.I64]) {
	is_jobdef_dead := (jobDef == nil) || (jobDef.jobType == nil) || jobDef.Disabled
	query_tasks := JobTaskJobRun.In(runningJobIds.ToAnys()...)
	if is_jobdef_dead { //  the rare edge case: un-Done/un-Cancelled tasks still in DB for old now-disabled-or-deleted-from-config job def
		query_tasks = query_tasks.And(jobTaskState.In(Running, Pending))
	} else { // the usual case.
		query_tasks = query_tasks.And(jobTaskState.Equal(Running)).And(
			JobTaskStartTime.LessThan(time.Now().Add(-(time.Minute + (time.Second * time.Duration(If(jobDef.TimeoutSecsTaskRun == 0, yodb.U32(Timeout1Min.Seconds()), jobDef.TimeoutSecsTaskRun)))))))
	}

	task_updates := map[*JobTask][]q.F{}
	yodb.Each[JobTask](ctx, query_tasks, 0, nil, func(task *JobTask, enough *bool) {
		task_upd_fields := JobTaskFields(jobTaskState, JobTaskAttempts, JobTaskStartTime, JobTaskFinishTime)
		if is_jobdef_dead {
			task.state = yodb.Text(Cancelled)
			if (len(task.Attempts) > 0) && (task.Attempts[0].Err == nil) {
				task.Attempts[0].Err = context.Canceled
			}
		} else if (!task.markForRetryOrAsFailed(ctx)) && (len(task.Attempts) > 0) && (task.Attempts[0].Err == nil) {
			task.Attempts[0].Err = ErrTimedOut
		}
		if len(task_upd_fields) > 0 {
			task_updates[task] = task_upd_fields
		}
	})
	for task, task_upd_fields := range task_updates {
		Try(func() {
			yodb.Update[JobTask](ctx, task, nil, false, task_upd_fields...)
		}, nil)
	}
}

func (me *engine) runJobTasks() {
	ctx := NewCtxNonHttp(Timeout1Min, false, "")
	defer ctx.OnDone(func() {
		DoAfter(me.options.IntervalRunTasks, me.runJobTasks)
	})

	pending_tasks := yodb.FindMany[JobTask](ctx, jobTaskState.Equal(string(Pending)), me.options.FetchTasksToRun, nil)
	GoItems(pending_tasks, func(it *JobTask) {
		me.runTask(ctx, it)
	}, me.options.MaxConcurrentOps)
}

func (me *engine) runTask(ctxForCacheReuse *Ctx, task *JobTask) {
	job_run := task.JobRun.Get(ctxForCacheReuse)
	job_def := job_run.jobDef(ctxForCacheReuse)
	timeout := Timeout1Min
	if (job_def != nil) && (job_def.TimeoutSecsTaskRun != 0) {
		timeout = time.Second * time.Duration(job_def.TimeoutSecsTaskRun)
	}
	ctx := ctxForCacheReuse.CopyButWith(timeout, true)
	defer ctx.OnDone(nil)

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
	if yodb.Update[JobTask](ctx, task, nil, false, task_upd_fields...) <= 0 {
		return // concurrently changed by sibling instance (note JobTask.OnBeforeStoring), likely in the same run attempt: bug out
	}

	ctx.DbTx(false)

	switch {
	case job_run == nil:
		task.Attempts[0].Err = errNotFoundJobRun(task.JobRun.Id())
	case job_def == nil:
		task.Attempts[0].Err = errNotFoundJobDef(job_run.Id)
	case job_def.jobType == nil:
		task.Attempts[0].Err = errNotFoundJobType(job_def.Name, job_def.JobTypeId)
	case !already_canceled: // actual RUNNING of task
		Try(func() {
			task.Results = job_def.jobType.TaskResults(job_run.ctx(ctx, task.Id), task.Details)
			jobType(string(job_def.JobTypeId)).checkTypeTaskResults(task.Results)
			task_upd_fields = sl.With(task_upd_fields, jobTaskResults.F())
		}, func(err any) {
			if task.Attempts[0].Err == nil {
				if task.Attempts[0].Err, _ = err.(error); task.Attempts[0].Err == nil {
					task.Attempts[0].Err = errors.New(str.FmtV(err))
				}
			}
		})
	}

	task.state, task.FinishTime =
		yodb.Text(If(already_canceled, Cancelled, Done)), yodb.DtNow()
	err_ctx := ctx.Err()
	did_mark_for_retry := false
	if err_ctx != nil && errors.Is(err_ctx, context.Canceled) {
		task.state = yodb.Text(Cancelled)
	} else if (!already_canceled) &&
		(((err_ctx != nil) && ((err_ctx.Error() == ErrTimedOut.Error()) || errors.Is(err_ctx, context.DeadlineExceeded))) ||
			((task.Attempts[0].Err != nil) && (job_def != nil) && (job_def.jobType != nil))) {
		did_mark_for_retry = task.markForRetryOrAsFailed(ctx)
	}
	if task.Attempts[0].Err == nil {
		task.Attempts[0].Err = err_ctx
	}
	if (task.Attempts[0].Err != nil) && did_mark_for_retry {
		task_upd_fields = sl.With(task_upd_fields, JobTaskStartTime.F())
	}

	// ready to save
	task_upd_fields = sl.Without(task_upd_fields, JobTaskStartTime.F())
	yodb.Update[JobTask](ctx, task, nil, false, task_upd_fields...)
}
