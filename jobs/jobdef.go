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

type JobSpec struct {
	Resource `yaml:",inline" json:",inline" bson:",inline"`

	HandlerID       string      `yaml:"handlerID" json:"handler_id" bson:"handler_id"`
	DisplayName     string      `yaml:"displayName" json:"display_name,omitempty" bson:"display_name,omitempty"`
	Disabled        bool        `yaml:"disabled" json:"disabled,omitempty" bson:"disabled,omitempty"`
	Schedules       []*Schedule `yaml:"schedules" json:"schedules,omitempty" bson:"schedules,omitempty"`
	AllowManualJobs bool        `yaml:"allowManualJobs" json:"allow_manual_jobs" bson:"allow_manual_jobs"`
	Timeouts        struct {
		JobPrepAndFinalize time.Duration `yaml:"jobPrepAndFinalize" json:"job_prep_finalize" bson:"job_prep_finalize"`
		TaskRun            time.Duration `yaml:"taskRun" json:"task_run" bson:"task_run"`
	} `yaml:"timeouts" json:"timeouts" bson:"timeouts"`
	TaskRetries             int            `yaml:"taskRetries" json:"task_retries,omitempty" bson:"task_retries,omitempty"`
	TaskResultsShrinkDownTo []string       `yaml:"taskResultsShrinkDownTo" json:"task_results_shrink_to,omitempty" bson:"task_results_shrink_to,omitempty"`
	DeleteAfterDays         int            `yaml:"deleteAfterDays" json:"delete_after_days,omitempty" bson:"delete_after_days,omitempty"`
	LogJobLifecycleEvents   *bool          `yaml:"logJobLifecycleEvents" json:"log_job_lifecycle_events,omitempty" bson:"log_job_lifecycle_events,omitempty"`
	LogTaskLifecycleEvents  *bool          `yaml:"logTaskLifecycleEvents" json:"log_task_lifecycle_events,omitempty" bson:"log_task_lifecycle_events,omitempty"`
	DefaultJobDetails       map[string]any `yaml:"defaultJobDetails" json:"default_job_details,omitempty" bson:"default_job_details,omitempty"`

	handler Handler
}

func (it *JobSpec) defaultJobDetails() (details JobDetails, err error) {
	if len(it.DefaultJobDetails) > 0 {
		details, _ = handler(it.HandlerID).wellTypedJobDetails(nil)
		err = ensureValueFromMap(&it.DefaultJobDetails, &details)
	}
	return
}

type Schedule struct {
	Disabled bool   `yaml:"disabled" json:"disabled,omitempty" bson:"disabled,omitempty"`
	Crontab  string `yaml:"rule" json:"rule,omitempty" bson:"rule,omitempty"`

	crontab crontab.Expr
}

func (it *JobSpec) EnsureValidOrErrorIfEnabled() (*JobSpec, error) {
	for _, err := range it.EnsureValid() {
		if !it.Disabled {
			return nil, err
		}
	}
	return it, nil
}

func (it *JobSpec) EnsureValid() (errs []error) { // not quite the same as "validation"  =)
	if handlerReg := handler(it.HandlerID); it.handler == nil && (!it.Disabled) && handlerReg != nil {
		it.handler = handlerReg.For(it.HandlerID)
	}
	if it.handler == nil && !it.Disabled {
		errs = append(errs, errNotFoundHandler(it.ID, it.HandlerID))
	}
	it.Timeouts.TaskRun = Clamp(11*time.Second, 22*time.Hour, it.Timeouts.TaskRun)
	it.Timeouts.JobPrepAndFinalize = Clamp(22*time.Second, 11*time.Hour, it.Timeouts.JobPrepAndFinalize)
	it.TaskRetries = Clamp(0, 1234, it.TaskRetries)
	it.DeleteAfterDays = Clamp(0, math.MaxInt32, it.DeleteAfterDays)
	for i, sched := range it.Schedules {
		if sched.Crontab = strings.TrimSpace(sched.Crontab); sched.Crontab == "" {
			errs = append(errs, errors.New(str.Fmt("job spec '%s' schedule %d/%d requires a `rule`", it, i+1, len(it.Schedules))))
		} else if sched.crontab == nil {
			if crontab, err := crontab.Parse(sched.Crontab); err != nil {
				errs = append(errs, errors.New(str.Fmt("job spec '%s' schedule %d/%d syntax error in '%s': %s", it, i+1, len(it.Schedules), sched.Crontab, err)))
			} else {
				sched.crontab = crontab
			}
		}
	}
	return
}

func (it *JobSpec) hasAnySchedulesEnabled() bool {
	return sl.HasWhere(it.Schedules, func(s *Schedule) bool { return !s.Disabled })
}

func (it *JobSpec) findClosestToNowSchedulableTimeSince(after *time.Time, alwaysPreferOverdue bool) *time.Time {
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
	// Especially useful for daily/weekly schedules that'd by unlucky chance fall into such a rare situation.
	return If(now.Sub(*past) < future.Sub(now), past, future)
}

func (it *JobSpec) ok(t time.Time) bool {
	if it.Disabled {
		return false
	}
	for _, schedule := range it.Schedules {
		if schedule.Disabled {
			continue
		}
		if schedule.crontab.DateAndTimeOK(t) {
			return true
		}
	}
	return false
}
