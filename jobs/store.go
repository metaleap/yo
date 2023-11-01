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

		GetJobRun(ctx context.Context, id string) (*Job, error)
		FindJobRun(ctx context.Context, filter *JobRunFilter, sort ...Sorting) (*Job, error)
		ListJobRuns(ctx context.Context, req ListRequest, filter *JobRunFilter) ([]*Job, string, error)
		CountJobRuns(ctx context.Context, limit int64, filter *JobRunFilter) (int64, error)
		InsertJobRuns(ctx context.Context, objs ...*Job) error
		DeleteJobRuns(ctx context.Context, filter *JobRunFilter) error

		GetJobTask(ctx context.Context, id string) (*Task, error)
		FindJobTask(ctx context.Context, filter *JobTaskFilter) (*Task, error)
		ListJobTasks(ctx context.Context, req ListRequest, filter *JobTaskFilter) ([]*Task, string, error)
		CountJobTasks(ctx context.Context, limit int64, filter *JobTaskFilter) (int64, error)
		InsertJobTasks(ctx context.Context, objs ...*Task) error
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
	Resource struct {
		Id string `json:"id"`
	}

	// store is the package-internal shim around `Backend` that all code in this
	// package is to use instead of directly calling `Backend`s method implementations.
	// Most shim methods are not mere call wraps but add needed checks or logic.
	store struct{ impl Store }
)

func R(id string) Resource         { return Resource{Id: id} }
func (it Resource) GetId() string  { return it.Id } //nolint:revive
func (it Resource) String() string { return it.Id }

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

func (it store) getJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, id string) (*Job, error) {
	job_run, err := it.impl.GetJobRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if loadDef && (job_run != nil) {
		if job_run.def, err = it.getJobDef(ctx, job_run.Def); (err != nil) && !mustLoadDef {
			err = nil
		}
	}
	return job_run, err
}

func (it store) findJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, filter *JobRunFilter) (*Job, error) {
	job_run, err := it.impl.FindJobRun(ctx, filter, defaultJobSorting...)
	if err != nil {
		return nil, err
	}
	if loadDef && (job_run != nil) {
		if job_run.def, err = it.getJobDef(ctx, job_run.Def); (err != nil) && !mustLoadDef {
			err = nil
		}
	}
	return job_run, err
}

func (it store) listJobRuns(ctx context.Context, loadDefs bool, mustLoadDefs bool, req ListRequest, filter *JobRunFilter) (jobRuns []*Job, jobDefs []*JobDef, pageTok string, err error) {
	if len(req.Sort) == 0 {
		req.Sort = defaultJobSorting
	}
	jobRuns, pageTok, err = it.impl.ListJobRuns(ctx, req, filter)
	if (err == nil) && loadDefs {
		if jobDefs, err = it.listJobDefs(ctx, JobDefFilter{}.WithIds(sl.To(jobRuns, func(v *Job) string { return v.Def })...)); err == nil {
			for _, job := range jobRuns {
				if job.def = findByID(jobDefs, job.Def); (job.def == nil) && mustLoadDefs {
					return nil, nil, "", errNotFoundDef(job.Def)
				}
			}
		} else if !mustLoadDefs {
			err = nil
		}
	}
	return
}

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it store) getJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, id string) (*Task, error) {
	job_task, err := it.impl.GetJobTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if loadJobRun && (job_task != nil) {
		if job_task.job, err = it.getJobRun(ctx, true, mustLoadJobRun, job_task.Job); err != nil && !mustLoadJobRun {
			err = nil
		}
	}
	return job_task, err
}

func (it store) findJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, filter *JobTaskFilter) (*Task, error) {
	job_task, err := it.impl.FindJobTask(ctx, filter)
	if err != nil {
		return nil, err
	}
	if loadJobRun && (job_task != nil) {
		if job_task.job, err = it.getJobRun(ctx, true, mustLoadJobRun, job_task.Job); (err != nil) && !mustLoadJobRun {
			err = nil
		}
	}
	return job_task, err
}

func (it store) listJobTasks(ctx context.Context, loadJobRuns bool, mustLoadJobRuns bool, req ListRequest, filter *JobTaskFilter) (jobTasks []*Task, jobRuns []*Job, jobDefs []*JobDef, pageTok string, err error) {
	jobTasks, pageTok, err = it.impl.ListJobTasks(ctx, req, filter)
	if (err == nil) && loadJobRuns {
		if jobRuns, jobDefs, _, err = it.listJobRuns(ctx, true, mustLoadJobRuns, ListRequest{PageSize: len(jobTasks)}, JobRunFilter{}.WithIds(sl.To(jobTasks, func(v *Task) string { return v.Job })...)); err == nil {
			for _, job_task := range jobTasks {
				if job_task.job = findByID(jobRuns, job_task.Job); (job_task.job == nil) && mustLoadJobRuns {
					return nil, nil, nil, "", errNotFoundJob(job_task.Job)
				}
			}
		} else if !mustLoadJobRuns {
			err = nil
		}
	}
	return
}

func (it store) saveJobRun(ctx context.Context, obj *Job) error {
	version := obj.ResourceVersion
	obj.ResourceVersion++
	return it.impl.SaveGuarded(ctx, obj, version)
}

func (it store) saveJobTask(ctx context.Context, obj *Task) error {
	version := obj.ResourceVersion
	obj.ResourceVersion++
	return it.impl.SaveGuarded(ctx, obj, version)
}

// below the so-far unaugmented, merely pass-through call wrappers, for completeness' sake: when future augmentation is needed, they're already here in place.

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it store) countJobRuns(ctx context.Context, limit int64, filter *JobRunFilter) (int64, error) {
	return it.impl.CountJobRuns(ctx, limit, filter)
}
func (it store) insertJobRuns(ctx context.Context, objs ...*Job) error {
	return it.impl.InsertJobRuns(ctx, objs...)
}
func (it store) deleteJobRuns(ctx context.Context, filter *JobRunFilter) error {
	return it.impl.DeleteJobRuns(ctx, filter)
}
func (it store) countJobTasks(ctx context.Context, limit int64, filter *JobTaskFilter) (int64, error) {
	return it.impl.CountJobTasks(ctx, limit, filter)
}
func (it store) insertJobTasks(ctx context.Context, objs ...*Task) error {
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
