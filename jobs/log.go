package jobs

import (
	yolog "yo/log"
	. "yo/util"
	"yo/util/str"
)

type logger str.Dict

func (me logger) Infof(msg string, args ...any) {
	yolog.Println(msg, args...)
}

func loggerNew() logger {
	return If(IsDevMode, logger{}, nil)
}

func (it *engine) logLifecycleEvents(forTask bool, def *JobDef, job *Job, task *Task) bool {
	if !IsDevMode {
		return false
	}
	if job == nil && task != nil {
		job = task.job
	}
	if def == nil && job != nil {
		def = job.def
	}
	if def != nil {
		if setting := If(forTask, def.LogTaskLifecycleEvents, def.LogJobLifecycleEvents); setting != nil {
			return *setting
		}
	}
	return If(forTask, it.options.LogTaskLifecycleEvents, it.options.LogJobLifecycleEvents)
}

func (it *Task) logger(log logger) logger {
	return logFor(log, nil, nil, it)
}

func (it *Job) logger(log logger) logger {
	return logFor(log, nil, it, nil)
}

func (it *JobDef) logger(log logger) logger {
	return logFor(log, it, nil, nil)
}

func logFor(log logger, jobDef *JobDef, job *Job, task *Task) logger {
	if !IsDevMode {
		return log
	}
	if job == nil && task != nil {
		job = task.job
	}
	if jobDef == nil && job != nil {
		jobDef = job.def
	}
	if jobDef != nil {
		log["job_def"], log["job_type"] = jobDef.Id, jobDef.HandlerId
	}
	if job != nil {
		log["job_def"], log["job_type"], log["job_id"], log["job_cancellation_reason"] = job.Def, job.HandlerID, job.Id, string(job.Info.CancellationReason)
	}
	if task != nil {
		log["job_type"], log["job_id"], log["job_task"] = task.HandlerID, task.Job, task.Id
	}
	return log
}

func (it *engine) logErr(log logger, err error, objs ...interface {
	logger(logger) logger
}) error {
	if (!IsDevMode) || it.backend.isVersionConflictDuringSave(err) {
		// we don't noise up the logs (or otherwise handle err) just because another pod
		// beat us to what both were attempting at the same time — it's by-design ignored
		return err
	}
	if err != nil {
		for _, obj := range objs {
			log = obj.logger(log)
		}
		log["err"] = err.Error()
		yolog.Println(str.FmtV(log))
	}
	return err
}
