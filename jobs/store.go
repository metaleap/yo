package jobs

import (
	"context"

	"yo/util/sl"
)

type (
	// Store (implementation provided by the importer) is for the raw CRUD backing storage.
	Store interface {
		GetJobDef(ctx context.Context, id string) (*JobDef, error)
		FindJobDef(ctx context.Context, filter *JobDefFilter) (*JobDef, error)
		ListJobDefs(ctx context.Context, filter *JobDefFilter) ([]*JobDef, error)

		GetJobRun(ctx context.Context, id string) (*JobRun, error)
		FindJobRun(ctx context.Context, filter *JobRunFilter, sort ...Sorting) (*JobRun, error)
		ListJobRuns(ctx context.Context, req ListRequest, filter *JobRunFilter) ([]*JobRun, string, error)
		CountJobRuns(ctx context.Context, limit int64, filter *JobRunFilter) (int64, error)
		InsertJobRuns(ctx context.Context, objs ...*JobRun) error
		DeleteJobRuns(ctx context.Context, filter *JobRunFilter) error

		GetJobTask(ctx context.Context, id string) (*JobTask, error)
		FindJobTask(ctx context.Context, filter *JobTaskFilter) (*JobTask, error)
		ListJobTasks(ctx context.Context, req ListRequest, filter *JobTaskFilter) ([]*JobTask, string, error)
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

	// store is the package-internal shim around `Backend` that all code in this
	// package is to use instead of directly calling `Backend`s method implementations.
	// Most shim methods are not mere call wraps but add needed checks or logic.
	store struct{ impl Store }
)

func (it store) getJobDef(ctx context.Context, id string) (*JobDef, error) {
	job_def, err := it.impl.GetJobDef(ctx, id)
	if (job_def != nil) && (err == nil) {
		return job_def.EnsureValidOrErrorIfEnabled()
	}
	return job_def, err
}

func (it store) findJobDef(ctx context.Context, filter *JobDefFilter) (*JobDef, error) {
	job_def, err := it.impl.FindJobDef(ctx, filter)
	if (job_def != nil) && (err == nil) {
		return job_def.EnsureValidOrErrorIfEnabled()
	}
	return job_def, err
}

func (it store) listJobDefs(ctx context.Context, filter *JobDefFilter) ([]*JobDef, error) {
	job_defs, err := it.impl.ListJobDefs(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, job_def := range job_defs {
		if _, err = job_def.EnsureValidOrErrorIfEnabled(); err != nil {
			return nil, err
		}
	}
	return job_defs, nil
}

var defaultJobSorting = []Sorting{{Key: "due_time", Value: -1}, {Key: "finish_time", Value: -1}, {Key: "start_time", Value: -1}, {Key: "_id", Value: -1}}

func (it store) getJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, id string) (*JobRun, error) {
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

func (it store) findJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, filter *JobRunFilter) (*JobRun, error) {
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

func (it store) listJobRuns(ctx context.Context, loadDefs bool, mustLoadDefs bool, req ListRequest, filter *JobRunFilter) (jobRuns []*JobRun, jobDefs []*JobDef, pageTok string, err error) {
	if len(req.Sort) == 0 {
		req.Sort = defaultJobSorting
	}
	jobRuns, pageTok, err = it.impl.ListJobRuns(ctx, req, filter)
	if (err == nil) && loadDefs {
		if jobDefs, err = it.listJobDefs(ctx, JobDefFilter{}.WithIds(sl.To(jobRuns, func(v *JobRun) string { return v.JobDefId })...)); err == nil {
			for _, job := range jobRuns {
				if job.jobDef = sl.FirstWhere(jobDefs, func(it *JobDef) bool { return (it.Id == job.JobDefId) }); (job.jobDef == nil) && mustLoadDefs {
					return nil, nil, "", errNotFoundJobDef(job.JobDefId)
				}
			}
		} else if !mustLoadDefs {
			err = nil
		}
	}
	return
}

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it store) getJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, id string) (*JobTask, error) {
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

func (it store) findJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, filter *JobTaskFilter) (*JobTask, error) {
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

func (it store) listJobTasks(ctx context.Context, loadJobRuns bool, mustLoadJobRuns bool, req ListRequest, filter *JobTaskFilter) (jobTasks []*JobTask, jobRuns []*JobRun, jobDefs []*JobDef, pageTok string, err error) {
	jobTasks, pageTok, err = it.impl.ListJobTasks(ctx, req, filter)
	if (err == nil) && loadJobRuns {
		if jobRuns, jobDefs, _, err = it.listJobRuns(ctx, true, mustLoadJobRuns, ListRequest{PageSize: len(jobTasks)}, JobRunFilter{}.WithIds(sl.To(jobTasks, func(v *JobTask) string { return v.JobRunId })...)); err == nil {
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

func (it store) saveJobRun(ctx context.Context, obj *JobRun) error {
	version := obj.Version
	obj.Version++
	return it.impl.SaveGuarded(ctx, obj, version)
}

func (it store) saveJobTask(ctx context.Context, obj *JobTask) error {
	version := obj.Version
	obj.Version++
	return it.impl.SaveGuarded(ctx, obj, version)
}

// below the so-far unaugmented, merely pass-through call wrappers, for completeness' sake: when future augmentation is needed, they're already here in place.

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it store) countJobRuns(ctx context.Context, limit int64, filter *JobRunFilter) (int64, error) {
	return it.impl.CountJobRuns(ctx, limit, filter)
}
func (it store) insertJobRuns(ctx context.Context, objs ...*JobRun) error {
	return it.impl.InsertJobRuns(ctx, objs...)
}
func (it store) deleteJobRuns(ctx context.Context, filter *JobRunFilter) error {
	return it.impl.DeleteJobRuns(ctx, filter)
}
func (it store) countJobTasks(ctx context.Context, limit int64, filter *JobTaskFilter) (int64, error) {
	return it.impl.CountJobTasks(ctx, limit, filter)
}
func (it store) insertJobTasks(ctx context.Context, objs ...*JobTask) error {
	return it.impl.InsertJobTasks(ctx, objs...)
}
func (it store) deleteJobTasks(ctx context.Context, filter *JobTaskFilter) error {
	return it.impl.DeleteJobTasks(ctx, filter)
}
func (it store) isVersionConflictDuringSave(err error) bool {
	return it.impl.IsVersionConflictDuringSaveGuarded(err)
}
func (it store) transacted(ctx context.Context, do func(context.Context) error) error {
	return it.impl.Transacted(ctx, do)
}
