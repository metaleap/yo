package jobs

import (
	"errors"
	"math"
	"strings"
	"time"

	"yo/jobs/crontab"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

type JobDef struct {
	Resource

	HandlerId          string
	DisplayName        string
	Disabled           bool
	Schedules          []*Schedule
	AllowManualJobRuns bool
	Timeouts           struct {
		JobPrepAndFinalize time.Duration
		TaskRun            time.Duration
	}
	TaskRetries             int
	TaskResultsShrinkDownTo []string
	DeleteAfterDays         int
	LogJobLifecycleEvents   *bool
	LogTaskLifecycleEvents  *bool
	DefaultJobDetails       map[string]any

	handler Handler
}

func (it *JobDef) defaultJobDetails() (details JobDetails, err error) {
	if len(it.DefaultJobDetails) > 0 {
		details, _ = handler(it.HandlerId).wellTypedJobDetails(nil)
		err = ensureValueFromMap(&it.DefaultJobDetails, &details)
	}
	return
}

type Schedule struct {
	Disabled bool   `yaml:"disabled" json:"disabled,omitempty" bson:"disabled,omitempty"`
	Crontab  string `yaml:"rule" json:"rule,omitempty" bson:"rule,omitempty"`

	crontab crontab.Expr
}

func (it *JobDef) EnsureValidOrErrorIfEnabled() (*JobDef, error) {
	for _, err := range it.EnsureValid() {
		if !it.Disabled {
			return nil, err
		}
	}
	return it, nil
}

func (it *JobDef) EnsureValid() (errs []error) { // not quite the same as "validation"  =)
	if handlerReg := handler(it.HandlerId); it.handler == nil && (!it.Disabled) && handlerReg != nil {
		it.handler = handlerReg.ById(it.HandlerId)
	}
	if it.handler == nil && !it.Disabled {
		errs = append(errs, errNotFoundHandler(it.Id, it.HandlerId))
	}
	it.Timeouts.TaskRun = Clamp(11*time.Second, 22*time.Hour, it.Timeouts.TaskRun)
	it.Timeouts.JobPrepAndFinalize = Clamp(22*time.Second, 11*time.Hour, it.Timeouts.JobPrepAndFinalize)
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
	const maxYearsInTheFuture = 123
	maxSearchDate := now.AddDate(maxYearsInTheFuture, 0, 0)
	for _, schedule := range it.Schedules {
		if schedule.Disabled {
			continue
		}
		p, f := schedule.crontab.SoonestTo(now, after, &maxSearchDate)
		if p != nil && (past == nil || p.After(*past)) {
			past = p
		}
		if f != nil && (future == nil || f.Before(*future)) {
			future = f
		}
	}
	if past == nil {
		return future
	} else if future == nil || (alwaysPreferOverdue && after != nil) {
		return past
	}
	// if the latest-possible-past-occurrence-after-last `past` is closer to Now than
	// the soonest-possible-future-occurrence `future`, the former is picked because
	// it means catching up on a missed past schedule (due to outages or such).
	// Edefially useful for daily/weekly schedules that'd by unlucky chance fall into such a rare situation.
	return If(now.Sub(*past) < future.Sub(now), past, future)
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
