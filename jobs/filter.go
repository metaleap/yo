package jobs

import (
	"strings"
	"time"

	. "yo/util"
	"yo/util/sl"
)

type Filter[T Resource] interface {
	Ok(T) bool
}

type JobDefFilter struct {
	Ids                []string
	DisplayName        *string
	Disabled           *bool
	StorageExpiry      *bool
	AllowManualJobRuns *bool
	EnabledSchedules   bool
	DisabledSchedules  bool
}

func (it JobDefFilter) WithAllowManualJobRuns(allowManualJobRuns bool) *JobDefFilter {
	it.AllowManualJobRuns = &allowManualJobRuns
	return &it
}

func (it JobDefFilter) WithIds(ids ...string) *JobDefFilter {
	it.Ids = sl.Uniq(ids)
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

func (it *JobDefFilter) Ok(cmp *JobDef) bool {
	if it == nil {
		return true
	}
	if (it.Disabled != nil && cmp.Disabled != *it.Disabled) ||
		(it.StorageExpiry != nil && *it.StorageExpiry != (cmp.DeleteAfterDays > 0)) ||
		(it.DisplayName != nil && !strings.Contains(strings.ToLower(cmp.DisplayName), strings.ToLower(*it.DisplayName))) ||
		(len(it.Ids) > 0 && !sl.Has(it.Ids, cmp.Id)) ||
		(it.EnabledSchedules && !cmp.hasAnySchedulesEnabled()) ||
		(it.DisabledSchedules && cmp.hasAnySchedulesEnabled()) ||
		(it.AllowManualJobRuns != nil && *it.AllowManualJobRuns != cmp.AllowManualJobRuns) {
		return false
	}
	return true
}

type JobRunFilter struct {
	Ids                      []string
	JobDefs                  []string
	JobTypes                 []string
	States                   []RunState
	AutoScheduled            *bool
	FinishedBefore           *time.Time
	ScheduledNextAfterJobRun string
	ResourceVersion          int
	Due                      *bool
}

func (it JobRunFilter) WithDue(due bool) *JobRunFilter {
	it.Due = &due
	return &it
}

func (it JobRunFilter) WithIds(ids ...string) *JobRunFilter {
	it.Ids = ids
	return &it
}

func (it JobRunFilter) WithAutoScheduled(autoScheduled bool) *JobRunFilter {
	it.AutoScheduled = &autoScheduled
	return &it
}

func (it JobRunFilter) WithScheduledNextAfterJobRun(scheduledNextAfterJobRun string) *JobRunFilter {
	it.ScheduledNextAfterJobRun = scheduledNextAfterJobRun
	return &it
}

func (it JobRunFilter) WithFinishedBefore(finishedBefore time.Time) *JobRunFilter {
	it.FinishedBefore = ToPtr(finishedBefore.In(Timezone))
	return &it
}

func (it JobRunFilter) WithJobDefs(jobDefs ...string) *JobRunFilter {
	it.JobDefs = jobDefs
	return &it
}

func (it JobRunFilter) WithJobTypes(jobTypes ...string) *JobRunFilter {
	it.JobTypes = jobTypes
	return &it
}

func (it JobRunFilter) WithStates(states ...RunState) *JobRunFilter {
	it.States = states
	return &it
}

func (it JobRunFilter) WithVersion(version int) *JobRunFilter {
	it.ResourceVersion = version
	return &it
}

func (it *JobRunFilter) Ok(cmp *JobRun) bool {
	if it == nil {
		return true
	}
	if (len(it.Ids) > 0 && !sl.Has(it.Ids, cmp.Id)) ||
		(len(it.JobDefs) > 0 && !sl.Has(it.JobDefs, cmp.JobDefId)) ||
		(len(it.JobTypes) > 0 && !sl.Has(it.JobTypes, cmp.JobTypeId)) ||
		(len(it.States) > 0 && !sl.Has(it.States, cmp.State)) ||
		(it.AutoScheduled != nil && *it.AutoScheduled != cmp.AutoScheduled) ||
		(it.FinishedBefore != nil && (cmp.FinishTime == nil || !cmp.FinishTime.Before(*it.FinishedBefore))) ||
		(it.Due != nil && *it.Due != cmp.DueTime.Before(*timeNow())) ||
		(it.ScheduledNextAfterJobRun != "" && it.ScheduledNextAfterJobRun != cmp.ScheduledNextAfterJobRun) ||
		(it.ResourceVersion != 0 && it.ResourceVersion != cmp.ResourceVersion) {
		return false
	}
	return true
}

type JobTaskFilter struct {
	Ids             []string
	JobRuns         []string
	JobTypes        []string
	States          []RunState
	StartedBefore   *time.Time
	Failed          *bool
	ResourceVersion int
}

func (it JobTaskFilter) WithIds(ids ...string) *JobTaskFilter {
	it.Ids = ids
	return &it
}

func (it JobTaskFilter) WithStates(states ...RunState) *JobTaskFilter {
	it.States = states
	return &it
}

func (it JobTaskFilter) WithJobRuns(jobRunIds ...string) *JobTaskFilter {
	it.JobRuns = sl.Without(jobRunIds, "", "*")
	return &it
}

func (it JobTaskFilter) WithJobTypes(jobTypes ...string) *JobTaskFilter {
	it.JobTypes = jobTypes
	return &it
}

func (it JobTaskFilter) WithVersion(version int) *JobTaskFilter {
	it.ResourceVersion = version
	return &it
}

func (it JobTaskFilter) WithFailed() *JobTaskFilter {
	failed := true
	it.Failed = &failed
	return &it
}

func (it JobTaskFilter) WithSucceeded() *JobTaskFilter {
	failed := false
	it.Failed = &failed
	return &it
}

func (it JobTaskFilter) WithStartedBefore(startedBefore time.Time) *JobTaskFilter {
	it.StartedBefore = ToPtr(startedBefore.In(Timezone))
	return &it
}

func (it *JobTaskFilter) Ok(cmp *JobTask) bool {
	if it == nil {
		return true
	}
	if (len(it.Ids) > 0 && !sl.Has(it.Ids, cmp.Id)) ||
		(len(it.JobRuns) > 0 && !sl.Has(it.JobRuns, cmp.JobRunId)) ||
		(len(it.JobTypes) > 0 && !sl.Has(it.JobTypes, cmp.JobTypeId)) ||
		(len(it.States) > 0 && !sl.Has(it.States, cmp.State)) ||
		(it.StartedBefore != nil && (cmp.StartTime == nil || !cmp.StartTime.Before(*it.StartedBefore))) ||
		(it.Failed != nil && !If(*it.Failed, cmp.Failed, cmp.Succeeded)()) ||
		(it.ResourceVersion != 0 && it.ResourceVersion != cmp.ResourceVersion) {
		return false
	}
	return true
}
