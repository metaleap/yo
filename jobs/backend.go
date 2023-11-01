package jobs

import (
	"context"

	"yo/util/sl"
)

type (
	// Backend (implementation provided by the importer) is for the raw CRUD backing storage.
	Backend interface {
		GetJobDef(ctx context.Context, id string) (*JobDef, error)
		FindJobDef(ctx context.Context, filter *JobDefFilter) (*JobDef, error)
		ListJobDefs(ctx context.Context, filter *JobDefFilter) ([]*JobDef, error)

		GetJobRun(ctx context.Context, id string) (*Job, error)
		FindJobRun(ctx context.Context, filter *JobFilter, sort ...Sorting) (*Job, error)
		ListJobRuns(ctx context.Context, req ListRequest, filter *JobFilter) ([]*Job, string, error)
		CountJobRuns(ctx context.Context, limit int64, filter *JobFilter) (int64, error)
		InsertJobRuns(ctx context.Context, objs ...*Job) error
		DeleteJobRuns(ctx context.Context, filter *JobFilter) error

		GetJobTask(ctx context.Context, id string) (*Task, error)
		FindJobTask(ctx context.Context, filter *TaskFilter) (*Task, error)
		ListJobTasks(ctx context.Context, req ListRequest, filter *TaskFilter) ([]*Task, string, error)
		CountJobTasks(ctx context.Context, limit int64, filter *TaskFilter) (int64, error)
		InsertJobTasks(ctx context.Context, objs ...*Task) error
		DeleteJobTasks(ctx context.Context, filter *TaskFilter) error

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

	// backend is the package-internal shim around `Backend` that all code in this
	// package is to use instead of directly calling `Backend`s method implementations.
	// Most shim methods are not mere call wraps but add needed checks or logic.
	backend struct{ impl Backend }
)

func R(id string) Resource         { return Resource{Id: id} }
func (it Resource) GetId() string  { return it.Id } //nolint:revive
func (it Resource) String() string { return it.Id }

func (it backend) getJobDef(ctx context.Context, id string) (*JobDef, error) {
	job_def, err := it.impl.GetJobDef(ctx, id)
	if (job_def != nil) && (err == nil) {
		return job_def.EnsureValidOrErrorIfEnabled()
	}
	return job_def, err
}

func (it backend) findJobDef(ctx context.Context, filter *JobDefFilter) (*JobDef, error) {
	job_def, err := it.impl.FindJobDef(ctx, filter)
	if (job_def != nil) && (err == nil) {
		return job_def.EnsureValidOrErrorIfEnabled()
	}
	return job_def, err
}

func (it backend) listJobDefs(ctx context.Context, filter *JobDefFilter) ([]*JobDef, error) {
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

func (it backend) getJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, id string) (*Job, error) {
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

func (it backend) findJobRun(ctx context.Context, loadDef bool, mustLoadDef bool, filter *JobFilter) (*Job, error) {
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

func (it backend) listJobRuns(ctx context.Context, loadDefs bool, mustLoadDefs bool, req ListRequest, filter *JobFilter) (jobRuns []*Job, jobDefs []*JobDef, pageTok string, err error) {
	if len(req.Sort) == 0 {
		req.Sort = defaultJobSorting
	}
	jobRuns, pageTok, err = it.impl.ListJobRuns(ctx, req, filter)
	if (err == nil) && loadDefs {
		if jobDefs, err = it.listJobDefs(ctx, JobDefFilter{}.WithIDs(sl.To(jobRuns, func(v *Job) string { return v.Def })...)); err == nil {
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
func (it backend) getJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, id string) (*Task, error) {
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

func (it backend) findJobTask(ctx context.Context, loadJobRun bool, mustLoadJobRun bool, filter *TaskFilter) (*Task, error) {
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

func (it backend) listJobTasks(ctx context.Context, loadJobRuns bool, mustLoadJobRuns bool, req ListRequest, filter *TaskFilter) (jobTasks []*Task, jobRuns []*Job, jobDefs []*JobDef, pageTok string, err error) {
	jobTasks, pageTok, err = it.impl.ListJobTasks(ctx, req, filter)
	if (err == nil) && loadJobRuns {
		if jobRuns, jobDefs, _, err = it.listJobRuns(ctx, true, mustLoadJobRuns, ListRequest{PageSize: len(jobTasks)}, JobFilter{}.WithIDs(sl.To(jobTasks, func(v *Task) string { return v.Job })...)); err == nil {
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

func (it backend) saveJobRun(ctx context.Context, obj *Job) error {
	version := obj.ResourceVersion
	obj.ResourceVersion++
	return it.impl.SaveGuarded(ctx, obj, version)
}

func (it backend) saveJobTask(ctx context.Context, obj *Task) error {
	version := obj.ResourceVersion
	obj.ResourceVersion++
	return it.impl.SaveGuarded(ctx, obj, version)
}

// below the so-far unaugmented, merely pass-through call wrappers, for completeness' sake: when future augmentation is needed, they're already here in place.

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it backend) countJobRuns(ctx context.Context, limit int64, filter *JobFilter) (int64, error) {
	return it.impl.CountJobRuns(ctx, limit, filter)
}
func (it backend) insertJobRuns(ctx context.Context, objs ...*Job) error {
	return it.impl.InsertJobRuns(ctx, objs...)
}
func (it backend) deleteJobRuns(ctx context.Context, filter *JobFilter) error {
	return it.impl.DeleteJobRuns(ctx, filter)
}
func (it backend) countJobTasks(ctx context.Context, limit int64, filter *TaskFilter) (int64, error) {
	return it.impl.CountJobTasks(ctx, limit, filter)
}
func (it backend) insertJobTasks(ctx context.Context, objs ...*Task) error {
	return it.impl.InsertJobTasks(ctx, objs...)
}
func (it backend) deleteJobTasks(ctx context.Context, filter *TaskFilter) error {
	return it.impl.DeleteJobTasks(ctx, filter)
}
func (it backend) isVersionConflictDuringSave(err error) bool {
	return it.impl.IsVersionConflictDuringSaveGuarded(err)
}
func (it backend) transacted(ctx context.Context, do func(context.Context) error) error {
	return it.impl.Transacted(ctx, do)
}
