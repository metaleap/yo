package yo_jobs_old

import (
	"errors"
	"math"
	"strings"
	"time"

	"yo/jobs_old/crontab"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

type JobDef struct {
	Id string

	JobTypeId          string
	DisplayName        string
	Disabled           bool
	Schedules          []*Schedule
	AllowManualJobRuns bool
	Timeouts           struct {
		JobRunPrepAndFinalize time.Duration
		TaskRun               time.Duration
	}
	TaskRetries            int
	DeleteAfterDays        int
	LogJobLifecycleEvents  *bool
	LogTaskLifecycleEvents *bool
	DefaultJobDetails      map[string]any

	jobType JobType
}

func (it *JobDef) defaultJobDetails() (details JobDetails, err error) {
	if len(it.DefaultJobDetails) > 0 {
		details, _ = jobType(it.JobTypeId).wellTypedJobDetails(nil)
		err = ensureValueFromMap(&it.DefaultJobDetails, &details)
	}
	return
}

type Schedule struct {
	Disabled bool
	Crontab  string

	crontab crontab.Expr
}

func (it *JobDef) EnsureValidOrErrorIfEnabled() (*JobDef, error) {
	for _, err := range it.EnsureValid() {
		if !it.Disabled { // dont hoist out of loop , need the above call in any case
			return nil, err
		}
	}
	return it, nil
}

func (it *JobDef) EnsureValid() (errs []error) { // a mix of sanitization and validation really
	if job_type_reg := jobType(it.JobTypeId); (it.jobType == nil) && (!it.Disabled) && (job_type_reg != nil) {
		it.jobType = job_type_reg.ById(it.JobTypeId)
	}
	if (it.jobType == nil) && !it.Disabled {
		errs = append(errs, errNotFoundJobType(it.Id, it.JobTypeId))
	}
	it.Timeouts.TaskRun = Clamp(11*time.Second, 22*time.Hour, it.Timeouts.TaskRun)
	it.Timeouts.JobRunPrepAndFinalize = Clamp(22*time.Second, 11*time.Hour, it.Timeouts.JobRunPrepAndFinalize)
	it.TaskRetries = Clamp(0, 1234, it.TaskRetries)
	it.DeleteAfterDays = Clamp(0, math.MaxInt32, it.DeleteAfterDays)
	for i, sched := range it.Schedules {
		if sched.Crontab = strings.TrimSpace(sched.Crontab); sched.Crontab == "" {
			errs = append(errs, errors.New(str.Fmt("job def '%s' schedule %d/%d requires a `rule`", it, i+1, len(it.Schedules))))
		} else if sched.crontab == nil {
			if crontab, err := crontab.Parse(sched.Crontab); err != nil {
				errs = append(errs, errors.New(str.Fmt("job def '%s' schedule %d/%d syntax error in '%s': %s", it, i+1, len(it.Schedules), sched.Crontab, err)))
			} else {
				sched.crontab = crontab
			}
		}
	}
	return
}

func (it *JobDef) hasAnySchedulesEnabled() bool {
	return sl.HasWhere(it.Schedules, func(s *Schedule) bool { return !s.Disabled })
}

func (it *JobDef) findClosestToNowSchedulableTimeSince(after *time.Time, alwaysPreferOverdue bool) *time.Time {
	if it.Disabled {
		return nil
	}
	now := *timeNow()
	var future, past *time.Time
	const max_years_in_the_future = 77
	max_search_date := now.AddDate(max_years_in_the_future, 0, 0)
	for _, schedule := range it.Schedules {
		if schedule.Disabled {
			continue
		}
		past_find, fut_find := schedule.crontab.SoonestTo(now, after, &max_search_date)
		if (past_find != nil) && ((past == nil) || past_find.After(*past)) {
			past = past_find
		}
		if (fut_find != nil) && ((future == nil) || fut_find.Before(*future)) {
			future = fut_find
		}
	}
	if past == nil {
		return future
	} else if future == nil || (alwaysPreferOverdue && (after != nil)) {
		return past
	}
	// if the latest-possible-past-occurrence-after-last `past` is closer to Now than
	// the soonest-possible-future-occurrence `future`, the former is picked because
	// it means catching up on a missed past schedule (due to outages or such).
	// Especially useful for daily/weekly schedules that'd by unlucky chance fall into such a rare situation.
	return If((now.Sub(*past) < future.Sub(now)), past, future)
}

func (it *JobDef) ok(t time.Time) bool {
	if it.Disabled {
		return false
	}
	for _, schedule := range it.Schedules {
		if schedule.Disabled {
			continue
		}
		if schedule.crontab.DateAndTimeOk(t) {
			return true
		}
	}
	return false
}
