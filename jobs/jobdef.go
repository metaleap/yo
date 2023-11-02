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
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	Name                             yodb.Text
	JobTypeId                        yodb.Text
	Disabled                         yodb.Bool
	AllowManualJobRuns               yodb.Bool
	Schedules                        yodb.Arr[yodb.Text]
	TimeoutSecsTaskRun               yodb.U32
	TimeoutSecsJobRunPrepAndFinalize yodb.U32
	MaxTaskRetries                   yodb.U8
	DeleteAfterDays                  yodb.U16

	jobType   JobType
	schedules []crontab.Expr
}

func (me *JobDef) EnsureValidOrErrorIfEnabled() (*JobDef, error) {
	for _, err := range me.EnsureValid() {
		if !me.Disabled { // dont hoist the line out of loop , need the above call in any case
			return nil, err
		}
	}
	return me, nil
}

func (me *JobDef) EnsureValid() (errs []error) { // a mix of sanitization and validation really
	if job_type_reg := jobType(string(me.JobTypeId)); (me.jobType == nil) && (!me.Disabled) && (job_type_reg != nil) {
		me.jobType = job_type_reg.ById(string(me.JobTypeId))
	}
	if (me.jobType == nil) && !me.Disabled {
		errs = append(errs, errNotFoundJobType(me.Name, me.JobTypeId))
	}
	for i, sched := range me.Schedules {
		if sched.Set(str.Trim); sched == "" {
			errs = append(errs, errors.New(str.Fmt("job def '%s' schedule %d/%d requires a crontab expression", me, i+1, len(me.Schedules))))
		} else if me.schedules[i] == nil {
			if crontab, err := crontab.Parse(string(sched)); err != nil {
				errs = append(errs, errors.New(str.Fmt("job def '%s' schedule %d/%d syntax error in '%s': %s", me, i+1, len(me.Schedules), sched, err)))
			} else {
				me.schedules[i] = crontab
			}
		}
	}
	return
}

func (me *JobDef) findClosestToNowSchedulableTimeSince(after *time.Time, alwaysPreferOverdue bool) *time.Time {
	if me.Disabled {
		return nil
	}
	now := time.Now()
	var future, past *time.Time
	const max_years_in_the_future = 77
	max_search_date := now.AddDate(max_years_in_the_future, 0, 0)
	for _, schedule := range me.schedules {
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

func (me *JobDef) ok(t time.Time) bool {
	if me.Disabled {
		return false
	}
	for _, schedule := range me.schedules {
		if schedule.DateAndTimeOk(t) {
			return true
		}
	}
	return false
}

func (me *JobDef) OnAfterLoaded() {
	me.jobType = nil
	if job_type_reg := jobType(string(me.JobTypeId)); (!me.Disabled) && (job_type_reg != nil) {
		me.jobType = job_type_reg.ById(string(me.JobTypeId))
	}
}
func (me *JobDef) OnBeforeStoring() {}
