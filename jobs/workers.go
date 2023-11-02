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

func qSafe[TFld q.Field](id yodb.I64, versionField TFld, versionNr yodb.U32) q.Query {
	return yodb.ColID.Equal(id).And(versionField.Equal(versionNr))
}

func (me *engine) runJobTasks() {
	defer func() {
		_ = recover() // the odd, super-rare connectivity/db-restart/etc fail doesnt bother us here on our regular interval
		DoAfter(me.options.IntervalRunTasks, me.runJobTasks)
	}()

	ctx := NewCtxNonHttp(TimeoutLong, false, "")
	pending_tasks := yodb.FindMany[JobTask](ctx, jobTaskState.Equal(string(Pending)), me.options.FetchTasksToRun, nil)
	GoItems(pending_tasks, func(it *JobTask) {
		_ = it.jobDef(ctx) // preloads both jobRun and jobDef
	}, me.options.MaxConcurrentOps)

	// ...then run them
	GoItems(pending_tasks, me.runTask, me.options.MaxConcurrentOps)
}

func (me *engine) runTask(task *JobTask) {
	time_started := time.Now()
	defer func() {
		_ = recover() // the odd, super-rare connectivity/db-restart/etc fail doesnt bother us here on our regular interval
		if old_canceler := me.setTaskCanceler(task.Id, nil); old_canceler != nil {
			old_canceler()
		} // else: already cancelled by concurrent `finalizeCancelingJobs` call
	}()

	job_run := task.JobRun.Get(nil) // already preloaded by runJobTasks
	job_def := job_run.jobDef(nil)  // dito
	ctx := NewCtxNonHttp(task.Timeout(nil /*dito*/), true, "")
	ctx.DbTx()
	// first, attempt to reserve task for running vs. other pods
	already_canceled := (job_run == nil) || (job_run.State() == Cancelled) || (job_run.State() == JobRunCancelling) ||
		(job_def == nil) || bool(job_def.Disabled) || (job_def.jobType == nil) ||
		(task.JobTypeId != job_def.JobTypeId) || (job_run.JobTypeId != job_def.JobTypeId)

	task_old_version, task_upd_fields := task.Version, JobTaskFields(jobTaskState, JobTaskFinishTime, JobTaskAttempts, JobTaskVersion)
	task.Version, task.state, task.FinishTime, task.Attempts =
		task.Version+1, yodb.Text(If(already_canceled, Cancelled, Running)), nil, append([]TaskAttempt{taskAttempt()}, task.Attempts...)
	if task.StartTime == nil {
		task.StartTime, task_upd_fields = (*yodb.DateTime)(task.Attempts[0].t), sl.With(task_upd_fields, JobTaskStartTime.F())
	}
	if yodb.Update[JobTask](ctx, task, qSafe(task.Id, JobTaskVersion, task_old_version), false, task_upd_fields...) <= 0 {
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
		did_mark_for_retry = task.markForRetryOrAsFailed(job_def)
	}
	if task.Attempts[0].Err == nil {
		task.Attempts[0].Err = err_ctx
	}
	if (task.Attempts[0].Err != nil) && !did_mark_for_retry {
		task.Failed, task_upd_fields = true, sl.With(task_upd_fields, JobTaskFailed.F())
	}

	// ready to save
	task_old_version = task.Version
	task.Version, task_upd_fields = task.Version+1, sl.Without(task_upd_fields, JobTaskStartTime.F())
	did_store := (0 < yodb.Update[JobTask](ctx, task, qSafe(task.Id, JobTaskVersion, task_old_version), false, task_upd_fields...))
	if did_store && (me.eventHandlers.onJobTaskExecuted != nil) {
		me.eventHandlers.onJobTaskExecuted(task, time.Now().Sub(time_started))
	}
}
