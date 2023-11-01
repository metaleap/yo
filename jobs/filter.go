package jobs

import (
	"strings"
	"time"

	. "yo/util"
	"yo/util/sl"
)

type Filter[T Resource] interface {
	OK(T) bool
}

type JobDefFilter struct {
	IDs               []string
	DisplayName       *string
	Disabled          *bool
	StorageExpiry     *bool
	AllowManualJobs   *bool
	EnabledSchedules  bool
	DisabledSchedules bool
}

func (it JobDefFilter) WithAllowManualJobs(allowManualJobs bool) *JobDefFilter {
	it.AllowManualJobs = &allowManualJobs
	return &it
}

func (it JobDefFilter) WithIDs(ids ...string) *JobDefFilter {
	it.IDs = sl.Uniq(ids)
	return &it
}

func (it JobDefFilter) WithDisabled(disabled bool) *JobDefFilter {
	it.Disabled = &disabled
	return &it
}

func (it JobDefFilter) WithStorageExpiry(storageExpiry bool) *JobDefFilter {
	it.StorageExpiry = &storageExpiry
	return &it
}

func (it JobDefFilter) WithDisabledSchedules() *JobDefFilter {
	it.DisabledSchedules, it.EnabledSchedules = true, false
	return &it
}

func (it JobDefFilter) WithEnabledSchedules() *JobDefFilter {
	it.EnabledSchedules, it.DisabledSchedules = true, false
	return &it
}

func (it *JobDefFilter) OK(cmp *JobDef) bool {
	if it == nil {
		return true
	}
	if (it.Disabled != nil && cmp.Disabled != *it.Disabled) ||
		(it.StorageExpiry != nil && *it.StorageExpiry != (cmp.DeleteAfterDays > 0)) ||
		(it.DisplayName != nil && !strings.Contains(strings.ToLower(cmp.DisplayName), strings.ToLower(*it.DisplayName))) ||
		(len(it.IDs) > 0 && !sl.Has(it.IDs, cmp.Id)) ||
		(it.EnabledSchedules && !cmp.hasAnySchedulesEnabled()) ||
		(it.DisabledSchedules && cmp.hasAnySchedulesEnabled()) ||
		(it.AllowManualJobs != nil && *it.AllowManualJobs != cmp.AllowManualJobs) {
		return false
	}
	return true
}

type JobFilter struct {
	IDs                   []string
	JobDefs               []string
	JobTypes              []string
	States                []RunState
	AutoScheduled         *bool
	FinishedBefore        *time.Time
	ScheduledNextAfterJob string
	ResourceVersion       int
	Due                   *bool
}

func (it JobFilter) WithDue(due bool) *JobFilter {
	it.Due = &due
	return &it
}

func (it JobFilter) WithIDs(ids ...string) *JobFilter {
	it.IDs = ids
	return &it
}

func (it JobFilter) WithAutoScheduled(autoScheduled bool) *JobFilter {
	it.AutoScheduled = &autoScheduled
	return &it
}

func (it JobFilter) WithScheduledNextAfterJob(scheduledNextAfterJob string) *JobFilter {
	it.ScheduledNextAfterJob = scheduledNextAfterJob
	return &it
}

func (it JobFilter) WithFinishedBefore(finishedBefore time.Time) *JobFilter {
	it.FinishedBefore = ToPtr(finishedBefore.In(Timezone))
	return &it
}

func (it JobFilter) WithJobDefs(jobDefs ...string) *JobFilter {
	it.JobDefs = jobDefs
	return &it
}

func (it JobFilter) WithJobTypes(jobTypes ...string) *JobFilter {
	it.JobTypes = jobTypes
	return &it
}

func (it JobFilter) WithStates(states ...RunState) *JobFilter {
	it.States = states
	return &it
}

func (it JobFilter) WithVersion(version int) *JobFilter {
	it.ResourceVersion = version
	return &it
}

func (it *JobFilter) OK(cmp *Job) bool {
	if it == nil {
		return true
	}
	if (len(it.IDs) > 0 && !sl.Has(it.IDs, cmp.Id)) ||
		(len(it.JobDefs) > 0 && !sl.Has(it.JobDefs, cmp.Def)) ||
		(len(it.JobTypes) > 0 && !sl.Has(it.JobTypes, cmp.HandlerID)) ||
		(len(it.States) > 0 && !sl.Has(it.States, cmp.State)) ||
		(it.AutoScheduled != nil && *it.AutoScheduled != cmp.AutoScheduled) ||
		(it.FinishedBefore != nil && (cmp.FinishTime == nil || !cmp.FinishTime.Before(*it.FinishedBefore))) ||
		(it.Due != nil && *it.Due != cmp.DueTime.Before(*timeNow())) ||
		(it.ScheduledNextAfterJob != "" && it.ScheduledNextAfterJob != cmp.ScheduledNextAfterJob) ||
		(it.ResourceVersion != 0 && it.ResourceVersion != cmp.ResourceVersion) {
		return false
	}
	return true
}

type TaskFilter struct {
	IDs             []string   `json:"ids,omitempty" bson:"ids,omitempty"`
	Jobs            []string   `json:"jobs,omitempty" bson:"jobs,omitempty"`
	JobTypes        []string   `json:"job_types,omitempty" bson:"job_types,omitempty"`
	States          []RunState `json:"states,omitempty" bson:"states,omitempty"`
	StartedBefore   *time.Time `json:"start_before,omitempty" bson:"start_before,omitempty"`
	Failed          *bool      `json:"failed,omitempty" bson:"failed,omitempty"`
	ResourceVersion int        `json:"resource_version,omitempty" bson:"resource_version,omitempty"`
}

func (it TaskFilter) WithIDs(ids ...string) *TaskFilter {
	it.IDs = ids
	return &it
}

func (it TaskFilter) WithStates(states ...RunState) *TaskFilter {
	it.States = states
	return &it
}

func (it TaskFilter) WithJobs(jobIDs ...string) *TaskFilter {
	it.Jobs = sl.Without(jobIDs, "", "*")
	return &it
}

func (it TaskFilter) WithJobTypes(jobTypes ...string) *TaskFilter {
	it.JobTypes = jobTypes
	return &it
}

func (it TaskFilter) WithVersion(version int) *TaskFilter {
	it.ResourceVersion = version
	return &it
}

func (it TaskFilter) WithFailed() *TaskFilter {
	failed := true
	it.Failed = &failed
	return &it
}

func (it TaskFilter) WithSucceeded() *TaskFilter {
	failed := false
	it.Failed = &failed
	return &it
}

func (it TaskFilter) WithStartedBefore(startedBefore time.Time) *TaskFilter {
	it.StartedBefore = ToPtr(startedBefore.In(Timezone))
	return &it
}

func (it *TaskFilter) OK(cmp *Task) bool {
	if it == nil {
		return true
	}
	if (len(it.IDs) > 0 && !sl.Has(it.IDs, cmp.Id)) ||
		(len(it.Jobs) > 0 && !sl.Has(it.Jobs, cmp.Job)) ||
		(len(it.JobTypes) > 0 && !sl.Has(it.JobTypes, cmp.HandlerID)) ||
		(len(it.States) > 0 && !sl.Has(it.States, cmp.State)) ||
		(it.StartedBefore != nil && (cmp.StartTime == nil || !cmp.StartTime.Before(*it.StartedBefore))) ||
		(it.Failed != nil && !If(*it.Failed, cmp.Failed, cmp.Succeeded)()) ||
		(it.ResourceVersion != 0 && it.ResourceVersion != cmp.ResourceVersion) {
		return false
	}
	return true
}
