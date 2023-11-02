package yo_jobs_old

import (
	"context"

	. "yo/util"
	"yo/util/sl"
)

var defaultJobSorting = []Sorting{{Key: "due_time", Value: -1}, {Key: "finish_time", Value: -1}, {Key: "start_time", Value: -1}, {Key: "_id", Value: -1}}

type (
	// Storage (implementation provided by the importer) is for the raw CRUD backing storage.
	Storage interface {
		GetJobDef(ctx context.Context, id string) (*JobDef, error)
		FindJobDef(ctx context.Context, filter *JobDefFilter) (*JobDef, error)
		FindJobDefs(ctx context.Context, filter *JobDefFilter, on func(jobDef *JobDef, enough *bool)) error

		GetJobRun(ctx context.Context, id string) (*JobRun, error)
		FindJobRun(ctx context.Context, filter *JobRunFilter, sort ...Sorting) (*JobRun, error)
		FindJobRuns(ctx context.Context, filter *JobRunFilter, max int) ([]*JobRun, string, error)
		CountJobRuns(ctx context.Context, limit int64, filter *JobRunFilter) (int64, error)
		InsertJobRuns(ctx context.Context, objs ...*JobRun) error
		DeleteJobRuns(ctx context.Context, filter *JobRunFilter) error

		GetJobTask(ctx context.Context, id string) (*JobTask, error)
		FindJobTask(ctx context.Context, filter *JobTaskFilter) (*JobTask, error)
		FindJobTasks(ctx context.Context, filter *JobTaskFilter, max int) ([]*JobTask, string, error)
		CountJobTasks(ctx context.Context, limit int64, filter *JobTaskFilter) (int64, error)
		InsertJobTasks(ctx context.Context, objs ...*JobTask) error
		DeleteJobTasks(ctx context.Context, filter *JobTaskFilter) error

		SaveGuarded(ctx context.Context, jobOrTask any, whereVersion int) error
		IsVersionConflictDuringSaveGuarded(error) bool
		Transacted(ctx context.Context, do func(context.Context) error) error
	}
	ListRequest struct {
		PageSize  int       `json:"page_size,omitempty"`
		PageToken string    `json:"-"`
		Sort      []Sorting `json:"sort,omitempty"`
	}
	Sorting = struct {
		Key   string `json:"key,omitempty"`
		Value any    `json:"value,omitempty"`
	}

	// storage is the package-internal shim around `Storage` that all code in this
	// package is to use instead of directly calling `Storage`s method implementations.
	// Most shim methods are not mere call wraps but add needed checks or logic.
	storage struct{ impl Storage }
)

func (it storage) getJobDef(ctx context.Context, id string) (*JobDef, error) {
	job_def, err := it.impl.GetJobDef(ctx, id)
	if (job_def != nil) && (err == nil) {
		return job_def.EnsureValidOrErrorIfEnabled()
	}
	return job_def, err
}

func (it storage) findJobDef(ctx context.Context, filter *JobDefFilter) (*JobDef, error) {
	job_def, err := it.impl.FindJobDef(ctx, filter)
	if (job_def != nil) && (err == nil) {
		return job_def.EnsureValidOrErrorIfEnabled()
	}
	return job_def, err
}

func (it storage) findJobDefs(ctx context.Context, filter *JobDefFilter, on func(jobDef *JobDef, enough *bool)) (err error) {
	var err_inner error
	err = it.impl.FindJobDefs(ctx, filter, func(jobDef *JobDef, enough *bool) {
		if _, err_inner = jobDef.EnsureValidOrErrorIfEnabled(); err_inner != nil {
			*enough = true
		}
	})
	return If(err != nil, err, err_inner)
}

func (it storage) listJobDefs(ctx context.Context, filter *JobDefFilter, max int) (ret []*JobDef, err error) {
	ret = make([]*JobDef, 0, max)
	err = it.findJobDefs(ctx, filter, func(jobDef *JobDef, enough *bool) {
		if ret = append(ret, jobDef); (max > 0) && (len(ret) == max) {
			*enough = true
		}
	})
	return
}

func (it storage) getJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, id string) (*JobRun, error) {
	job_run, err := it.impl.GetJobRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if loadDef && (job_run != nil) {
		if job_run.jobDef, err = it.getJobDef(ctx, job_run.JobDefId); (err != nil) && !mustLoadDef {
			err = nil
		}
	}
	return job_run, err
}

func (it storage) findJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, filter *JobRunFilter) (*JobRun, error) {
	job_run, err := it.impl.FindJobRun(ctx, filter, defaultJobSorting...)
	if err != nil {
		return nil, err
	}
	if loadDef && (job_run != nil) {
		if job_run.jobDef, err = it.getJobDef(ctx, job_run.JobDefId); (err != nil) && !mustLoadDef {
			err = nil
		}
	}
	return job_run, err
}

func (it storage) findJobRuns(ctx context.Context, loadDefs bool, mustLoadDefs bool, filter *JobRunFilter, max int) (jobRuns []*JobRun, jobDefs []*JobDef, pageTok string, err error) {
	return
}

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it storage) getJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, id string) (*JobTask, error) {
	job_task, err := it.impl.GetJobTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if loadJobRun && (job_task != nil) {
		if job_task.jobRun, err = it.getJobRun(ctx, true, mustLoadJobRun, job_task.JobRunId); err != nil && !mustLoadJobRun {
			err = nil
		}
	}
	return job_task, err
}

func (it storage) findJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, filter *JobTaskFilter) (*JobTask, error) {
	job_task, err := it.impl.FindJobTask(ctx, filter)
	if err != nil {
		return nil, err
	}
	if loadJobRun && (job_task != nil) {
		if job_task.jobRun, err = it.getJobRun(ctx, true, mustLoadJobRun, job_task.JobRunId); (err != nil) && !mustLoadJobRun {
			err = nil
		}
	}
	return job_task, err
}

func (it storage) findJobTasks(ctx context.Context, loadJobRuns bool, mustLoadJobRuns bool, filter *JobTaskFilter, max int) (jobTasks []*JobTask, jobRuns []*JobRun, jobDefs []*JobDef, pageTok string, err error) {
	jobTasks, pageTok, err = it.impl.FindJobTasks(ctx, filter, max)
	if (err == nil) && loadJobRuns {
		if jobRuns, jobDefs, _, err = it.findJobRuns(ctx, true, mustLoadJobRuns, JobRunFilter{}.WithIds(sl.To(jobTasks, func(v *JobTask) string { return v.JobRunId })...), len(jobTasks)); err == nil {
			for _, job_task := range jobTasks {
				if job_task.jobRun = sl.FirstWhere(jobRuns, func(it *JobRun) bool { return (it.Id == job_task.JobRunId) }); (job_task.jobRun == nil) && mustLoadJobRuns {
					return nil, nil, nil, "", errNotFoundJobRun(job_task.JobRunId)
				}
			}
		} else if !mustLoadJobRuns {
			err = nil
		}
	}
	return
}

func (it storage) saveJobRun(ctx context.Context, obj *JobRun) error {
	version := obj.Version
	obj.Version++
	return it.impl.SaveGuarded(ctx, obj, version)
}

func (it storage) saveJobTask(ctx context.Context, obj *JobTask) error {
	version := obj.Version
	obj.Version++
	return it.impl.SaveGuarded(ctx, obj, version)
}

// below the so-far unaugmented, merely pass-through call wrappers, for completeness' sake: when future augmentation is needed, they're already here in place.

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it storage) countJobRuns(ctx context.Context, limit int64, filter *JobRunFilter) (int64, error) {
	return it.impl.CountJobRuns(ctx, limit, filter)
}
func (it storage) insertJobRuns(ctx context.Context, objs ...*JobRun) error {
	return it.impl.InsertJobRuns(ctx, objs...)
}
func (it storage) deleteJobRuns(ctx context.Context, filter *JobRunFilter) error {
	return it.impl.DeleteJobRuns(ctx, filter)
}
func (it storage) countJobTasks(ctx context.Context, limit int64, filter *JobTaskFilter) (int64, error) {
	return it.impl.CountJobTasks(ctx, limit, filter)
}
func (it storage) insertJobTasks(ctx context.Context, objs ...*JobTask) error {
	return it.impl.InsertJobTasks(ctx, objs...)
}
func (it storage) deleteJobTasks(ctx context.Context, filter *JobTaskFilter) error {
	return it.impl.DeleteJobTasks(ctx, filter)
}
func (it storage) isVersionConflictDuringSave(err error) bool {
	return it.impl.IsVersionConflictDuringSaveGuarded(err)
}
func (it storage) transacted(ctx context.Context, do func(context.Context) error) error {
	return it.impl.Transacted(ctx, do)
}
