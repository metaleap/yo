package jobs

import (
	"context"

	"yo/util/sl"
)

type (
	// Backend (implementation provided by the importer) is for the raw CRUD backing storage.
	Backend interface {
		GetJobSpec(ctx context.Context, id string) (*JobSpec, error)
		FindJobSpec(ctx context.Context, filter *JobSpecFilter) (*JobSpec, error)
		ListJobSpecs(ctx context.Context, filter *JobSpecFilter) ([]*JobSpec, error)

		GetJob(ctx context.Context, id string) (*Job, error)
		FindJob(ctx context.Context, filter *JobFilter, sort ...Sorting) (*Job, error)
		ListJobs(ctx context.Context, req ListRequest, filter *JobFilter) ([]*Job, string, error)
		CountJobs(ctx context.Context, limit int64, filter *JobFilter) (int64, error)
		InsertJobs(ctx context.Context, objs ...*Job) error
		DeleteJobs(ctx context.Context, filter *JobFilter) error

		GetTask(ctx context.Context, id string) (*Task, error)
		FindTask(ctx context.Context, filter *TaskFilter) (*Task, error)
		ListTasks(ctx context.Context, req ListRequest, filter *TaskFilter) ([]*Task, string, error)
		CountTasks(ctx context.Context, limit int64, filter *TaskFilter) (int64, error)
		InsertTasks(ctx context.Context, objs ...*Task) error
		DeleteTasks(ctx context.Context, filter *TaskFilter) error

		SaveGuarded(ctx context.Context, jobOrTask any, whereVersion int) error
		IsVersionConflictDuringSaveGuarded(error) bool
		Transacted(ctx context.Context, do func(context.Context) error) error
	}
	ListRequest struct {
		PageSize  int       `json:"page_size,omitempty" bson:"page_size,omitempty"`
		PageToken string    `json:"-" bson:"-"`
		Sort      []Sorting `json:"sort,omitempty" bson:"sort,omitempty"`
	}
	Sorting = struct {
		Key   string `json:"key,omitempty" bson:"key,omitempty"`
		Value any    `json:"value,omitempty" bson:"value,omitempty"`
	}
	Resource struct {
		ID string `bson:"id" yaml:"id" json:"id"`
	}

	// backend is the package-internal shim around `Backend` that all code in this
	// package is to use instead of directly calling `Backend`s method implementations.
	// Most shim methods are not mere call wraps but add needed checks or logic.
	backend struct{ impl Backend }
)

func R(id string) Resource         { return Resource{ID: id} }
func (it Resource) GetId() string  { return it.ID } //nolint:revive
func (it Resource) String() string { return it.ID }

func (it backend) getJobSpec(ctx context.Context, id string) (*JobSpec, error) {
	jobSpec, err := it.impl.GetJobSpec(ctx, id)
	if jobSpec != nil && err == nil {
		return jobSpec.EnsureValidOrErrorIfEnabled()
	}
	return jobSpec, err
}

func (it backend) findJobSpec(ctx context.Context, filter *JobSpecFilter) (*JobSpec, error) {
	jobSpec, err := it.impl.FindJobSpec(ctx, filter)
	if jobSpec != nil && err == nil {
		return jobSpec.EnsureValidOrErrorIfEnabled()
	}
	return jobSpec, err
}

func (it backend) listJobSpecs(ctx context.Context, filter *JobSpecFilter) ([]*JobSpec, error) {
	jobSpecs, err := it.impl.ListJobSpecs(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, jobSpec := range jobSpecs {
		if _, err = jobSpec.EnsureValidOrErrorIfEnabled(); err != nil {
			return nil, err
		}
	}
	return jobSpecs, nil
}

var defaultJobSorting = []Sorting{{Key: "due_time", Value: -1}, {Key: "finish_time", Value: -1}, {Key: "start_time", Value: -1}, {Key: "_id", Value: -1}}

func (it backend) getJob(ctx context.Context, loadSpec bool, mustLoadSpec bool, id string) (*Job, error) {
	job, err := it.impl.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if loadSpec && job != nil {
		if job.spec, err = it.getJobSpec(ctx, job.Spec); err != nil && !mustLoadSpec {
			err = nil
		}
	}
	return job, err
}

func (it backend) findJob(ctx context.Context, loadSpec bool, mustLoadSpec bool, filter *JobFilter) (*Job, error) {
	job, err := it.impl.FindJob(ctx, filter, defaultJobSorting...)
	if err != nil {
		return nil, err
	}
	if loadSpec && job != nil {
		if job.spec, err = it.getJobSpec(ctx, job.Spec); err != nil && !mustLoadSpec {
			err = nil
		}
	}
	return job, err
}

func (it backend) listJobs(ctx context.Context, loadSpecs bool, mustLoadSpecs bool, req ListRequest, filter *JobFilter) (jobs []*Job, jobSpecs []*JobSpec, pageTok string, err error) {
	if len(req.Sort) == 0 {
		req.Sort = defaultJobSorting
	}
	jobs, pageTok, err = it.impl.ListJobs(ctx, req, filter)
	if err == nil && loadSpecs {
		if jobSpecs, err = it.listJobSpecs(ctx, JobSpecFilter{}.WithIDs(sl.To(jobs, func(v *Job) string { return v.Spec })...)); err == nil {
			for _, job := range jobs {
				if job.spec = findByID(jobSpecs, job.Spec); job.spec == nil && mustLoadSpecs {
					return nil, nil, "", errNotFoundSpec(job.Spec)
				}
			}
		} else if !mustLoadSpecs {
			err = nil
		}
	}
	return
}

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it backend) getTask(ctx context.Context, loadJob bool, mustLoadJob bool, id string) (*Task, error) {
	task, err := it.impl.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if loadJob && task != nil {
		if task.job, err = it.getJob(ctx, true, mustLoadJob, task.Job); err != nil && !mustLoadJob {
			err = nil
		}
	}
	return task, err
}

func (it backend) findTask(ctx context.Context, loadJob bool, mustLoadJob bool, filter *TaskFilter) (*Task, error) {
	task, err := it.impl.FindTask(ctx, filter)
	if err != nil {
		return nil, err
	}
	if loadJob && task != nil {
		if task.job, err = it.getJob(ctx, true, mustLoadJob, task.Job); err != nil && !mustLoadJob {
			err = nil
		}
	}
	return task, err
}

func (it backend) listTasks(ctx context.Context, loadJobs bool, mustLoadJobs bool, req ListRequest, filter *TaskFilter) (tasks []*Task, jobs []*Job, jobSpecs []*JobSpec, pageTok string, err error) {
	tasks, pageTok, err = it.impl.ListTasks(ctx, req, filter)
	if err == nil && loadJobs {
		if jobs, jobSpecs, _, err = it.listJobs(ctx, true, mustLoadJobs, ListRequest{PageSize: len(tasks)}, JobFilter{}.WithIDs(sl.To(tasks, func(v *Task) string { return v.Job })...)); err == nil {
			for _, task := range tasks {
				if task.job = findByID(jobs, task.Job); task.job == nil && mustLoadJobs {
					return nil, nil, nil, "", errNotFoundJob(task.Job)
				}
			}
		} else if !mustLoadJobs {
			err = nil
		}
	}
	return
}

func (it backend) saveJob(ctx context.Context, obj *Job) error {
	version := obj.ResourceVersion
	obj.ResourceVersion++
	return it.impl.SaveGuarded(ctx, obj, version)
}

func (it backend) saveTask(ctx context.Context, obj *Task) error {
	version := obj.ResourceVersion
	obj.ResourceVersion++
	return it.impl.SaveGuarded(ctx, obj, version)
}

// below the so-far unaugmented, merely pass-through call wrappers, for completeness' sake: when future augmentation is needed, they're already here in place.

//nolint:unused // dev note: please leave it in. once it was needed, better to have it ready & in consistency with all the above & below
func (it backend) countJobs(ctx context.Context, limit int64, filter *JobFilter) (int64, error) {
	return it.impl.CountJobs(ctx, limit, filter)
}
func (it backend) insertJobs(ctx context.Context, objs ...*Job) error {
	return it.impl.InsertJobs(ctx, objs...)
}
func (it backend) deleteJobs(ctx context.Context, filter *JobFilter) error {
	return it.impl.DeleteJobs(ctx, filter)
}
func (it backend) countTasks(ctx context.Context, limit int64, filter *TaskFilter) (int64, error) {
	return it.impl.CountTasks(ctx, limit, filter)
}
func (it backend) insertTasks(ctx context.Context, objs ...*Task) error {
	return it.impl.InsertTasks(ctx, objs...)
}
func (it backend) deleteTasks(ctx context.Context, filter *TaskFilter) error {
	return it.impl.DeleteTasks(ctx, filter)
}
func (it backend) isVersionConflictDuringSave(err error) bool {
	return it.impl.IsVersionConflictDuringSaveGuarded(err)
}
func (it backend) transacted(ctx context.Context, do func(context.Context) error) error {
	return it.impl.Transacted(ctx, do)
}
