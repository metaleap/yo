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

func (it *engine) logLifecycleEvents(forTask bool, spec *JobSpec, job *Job, task *Task) bool {
	if !IsDevMode {
		return false
	}
	if job == nil && task != nil {
		job = task.job
	}
	if spec == nil && job != nil {
		spec = job.spec
	}
	if spec != nil {
		if setting := If(forTask, spec.LogTaskLifecycleEvents, spec.LogJobLifecycleEvents); setting != nil {
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

func (it *JobSpec) logger(log logger) logger {
	return logFor(log, it, nil, nil)
}

func logFor(log logger, jobSpec *JobSpec, job *Job, task *Task) logger {
	if !IsDevMode {
		return log
	}
	if job == nil && task != nil {
		job = task.job
	}
	if jobSpec == nil && job != nil {
		jobSpec = job.spec
	}
	if jobSpec != nil {
		log["job_spec"], log["job_type"] = jobSpec.ID, jobSpec.HandlerID
	}
	if job != nil {
		log["job_spec"], log["job_type"], log["job_id"], log["job_cancellation_reason"] = job.Spec, job.HandlerID, job.ID, string(job.Info.CancellationReason)
	}
	if task != nil {
		log["job_type"], log["job_id"], log["job_task"] = task.HandlerID, task.Job, task.ID
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
