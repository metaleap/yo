package yojobs

import (
	"time"

	yodb "yo/db"
	q "yo/db/query"
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

func (me *JobDef) id() yodb.I64 { return me.Id }

func (me *JobDef) findClosestToNowSchedulableTimeSince(after *time.Time, alwaysPreferOverdue bool) *time.Time {
	if me.Disabled || (len(me.schedules) == 0) {
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
	} else if (future == nil) || (alwaysPreferOverdue && (after != nil)) {
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

var _ yodb.Obj = (*JobDef)(nil)

func (me *JobDef) OnBeforeStoring() (q.Query, []q.F) { return nil, nil }
func (me *JobDef) OnAfterLoaded() {
	if job_type_reg := jobType(string(me.JobTypeId)); (!me.Disabled) && (job_type_reg != nil) {
		me.jobType = job_type_reg.ById(string(me.JobTypeId))
	}
	me.schedules = make([]crontab.Expr, len(me.Schedules))
	for i, sched := range me.Schedules {
		if sched.Set(str.Trim); sched == "" {
			panic(str.Fmt("job def '%s' schedule %d/%d requires a crontab expression", me, i+1, len(me.Schedules)))
		} else if crontab, err := crontab.Parse(string(sched)); err != nil {
			panic(str.Fmt("job def '%s' schedule %d/%d syntax error in '%s': %s", me, i+1, len(me.Schedules), sched, err))
		} else {
			me.schedules[i] = crontab
		}
	}
}
