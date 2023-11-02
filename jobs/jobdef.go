package yojobs

import (
	"errors"
	"time"

	yodb "yo/db"
	"yo/jobs/crontab"
	. "yo/util"
	"yo/util/str"
)

type JobDef struct {
	Id   yodb.I64
	Name yodb.Text

	JobTypeId                        yodb.Text
	Disabled                         yodb.Bool
	AllowManualJobRuns               yodb.Bool
	Schedules                        yodb.Arr[yodb.Text]
	TimeoutJobRunPrepAndFinalizeSecs yodb.U32
	TimeoutTaskRunSecs               yodb.U32
	MaxTaskRetries                   yodb.U8
	DeleteAfterDays                  yodb.U16

	jobType   JobType
	schedules []crontab.Expr
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
	if job_type_reg := jobType(string(it.JobTypeId)); (it.jobType == nil) && (!it.Disabled) && (job_type_reg != nil) {
		it.jobType = job_type_reg.ById(string(it.JobTypeId))
	}
	if (it.jobType == nil) && !it.Disabled {
		errs = append(errs, errNotFoundJobType(it.Name, it.JobTypeId))
	}
	for i, sched := range it.Schedules {
		if sched.Set(str.Trim); sched == "" {
			errs = append(errs, errors.New(str.Fmt("job def '%s' schedule %d/%d requires a crontab expression", it, i+1, len(it.Schedules))))
		} else if it.schedules[i] == nil {
			if crontab, err := crontab.Parse(string(sched)); err != nil {
				errs = append(errs, errors.New(str.Fmt("job def '%s' schedule %d/%d syntax error in '%s': %s", it, i+1, len(it.Schedules), sched, err)))
			} else {
				it.schedules[i] = crontab
			}
		}
	}
	return
}

func (it *JobDef) findClosestToNowSchedulableTimeSince(after *time.Time, alwaysPreferOverdue bool) *time.Time {
	if it.Disabled {
		return nil
	}
	now := *timeNow()
	var future, past *time.Time
	const max_years_in_the_future = 77
	max_search_date := now.AddDate(max_years_in_the_future, 0, 0)
	for _, schedule := range it.schedules {
		past_find, fut_find := schedule.SoonestTo(now, after, &max_search_date)
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
	for _, schedule := range it.schedules {
		if schedule.DateAndTimeOk(t) {
			return true
		}
	}
	return false
}
