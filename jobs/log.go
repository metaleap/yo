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

func (it *engine) logLifecycleEvents(jobDef *JobDef, jobRun *JobRun, jobTask *JobTask) bool {
	if !IsDevMode {
		return false
	}
	if (jobRun == nil) && (jobTask != nil) {
		jobRun = jobTask.jobRun
	}
	if (jobDef == nil) && (jobRun != nil) {
		jobDef = jobRun.jobDef
	}
	for_task := (jobTask != nil)
	if jobDef != nil {
		if setting := If(for_task, jobDef.LogTaskLifecycleEvents, jobDef.LogJobLifecycleEvents); setting != nil {
			return *setting
		}
	}
	return If(for_task, it.options.LogTaskLifecycleEvents, it.options.LogJobLifecycleEvents)
}

func (it *JobTask) logger(log logger) logger { return logFor(log, nil, nil, it) }
func (it *JobRun) logger(log logger) logger  { return logFor(log, nil, it, nil) }
func (it *JobDef) logger(log logger) logger  { return logFor(log, it, nil, nil) }

func logFor(log logger, jobDef *JobDef, jobRun *JobRun, jobTask *JobTask) logger {
	if !IsDevMode {
		return log
	}
	if (jobRun == nil) && (jobTask != nil) {
		jobRun = jobTask.jobRun
	}
	if (jobDef == nil) && (jobRun != nil) {
		jobDef = jobRun.jobDef
	}
	if jobDef != nil {
		log["job_def"], log["job_type"] = jobDef.Id, jobDef.JobTypeId
	}
	if jobRun != nil {
		log["job_def"], log["job_type"], log["job_id"], log["job_cancellation_reason"] = jobRun.JobDefId, jobRun.JobTypeId, jobRun.Id, string(jobRun.Info.CancellationReason)
	}
	if jobTask != nil {
		log["job_type"], log["job_id"], log["job_task"] = jobTask.JobTypeId, jobTask.JobRunId, jobTask.Id
	}
	return log
}

func (it *engine) logErr(log logger, err error, objs ...interface {
	logger(logger) logger
}) error {
	if (!IsDevMode) || it.storage.isVersionConflictDuringSave(err) {
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
