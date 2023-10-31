package jobs

import (
	"jobs/pkg/utils"

	"go.enpowerx.io/infrastructure/pkg/logger"

	"go.uber.org/zap"
)

func (it *engine) logLifecycleEvents(forTask bool, spec *JobSpec, job *Job, task *Task) bool {
	if job == nil && task != nil {
		job = task.job
	}
	if spec == nil && job != nil {
		spec = job.spec
	}
	if spec != nil {
		if setting := utils.If(forTask, spec.LogTaskLifecycleEvents, spec.LogJobLifecycleEvents); setting != nil {
			return *setting
		}
	}
	return utils.If(forTask, it.options.LogTaskLifecycleEvents, it.options.LogJobLifecycleEvents)
}

func logFor(log logger.Logger, jobSpec *JobSpec, job *Job, task *Task) logger.Logger {
	if job == nil && task != nil {
		job = task.job
	}
	if jobSpec == nil && job != nil {
		jobSpec = job.spec
	}
	zaps := make(map[string]string, 6)
	if jobSpec != nil {
		zaps["tenant"], zaps["job_spec"], zaps["job_type"] = jobSpec.Tenant, jobSpec.ID, jobSpec.HandlerID
	}
	if job != nil {
		zaps["tenant"], zaps["job_spec"], zaps["job_type"], zaps["job_id"], zaps["job_cancellation_reason"] = job.Tenant, job.Spec, job.HandlerID, job.ID, string(job.Info.CancellationReason)
	}
	if task != nil {
		zaps["tenant"], zaps["job_type"], zaps["job_id"], zaps["job_task"] = task.Tenant, task.HandlerID, task.Job, task.ID
	}
	for k, v := range zaps {
		if v != "" {
			log = log.With(zap.String(k, v))
		}
	}
	return log
}

func (it *Task) logger(log logger.Logger) logger.Logger {
	return logFor(log, nil, nil, it)
}

func (it *Job) logger(log logger.Logger) logger.Logger {
	return logFor(log, nil, it, nil)
}

func (it *JobSpec) logger(log logger.Logger) logger.Logger {
	return logFor(log, it, nil, nil)
}

func (it *engine) logErr(log logger.Logger, err error, objs ...interface {
	logger(logger.Logger) logger.Logger
}) error {
	if it.backend.isVersionConflictDuringSave(err) {
		// we don't noise up the logs (or otherwise handle err) just because another pod
		// beat us to what both were attempting at the same time — it's by design supported
		return err
	}
	if err != nil {
		if log == nil {
			log = logger.Background()
		}
		for _, obj := range objs {
			log = obj.logger(log)
		}
		log.WithError(err).Errorf(err.Error())
	}
	return err
}
