package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	. "yo/util"
)

type RunState string

const (
	RunStateUnspecified RunState = ""
	Pending             RunState = "PENDING"
	Running             RunState = "RUNNING"
	Done                RunState = "DONE"
	Cancelled           RunState = "CANCELLED"
	// Cancelling only exists for `Job`s, never for `Task`s.
	Cancelling RunState = "CANCELLING"
)

type CancellationReason string

const (
	CancellationReasonDuplicate            CancellationReason = "job_duplicate"
	CancellationReasonSpecInvalidOrGone    CancellationReason = "jobspec_invalid_or_gone"
	CancellationReasonSpecChanged          CancellationReason = "jobspec_changed"
	CancellationReasonJobTypeInvalidOrGone CancellationReason = "jobtype_invalid_or_gone"
)

type Job struct {
	Resource `json:",inline" bson:",inline"`

	HandlerID     string     `json:"job_type" bson:"job_type"`
	Spec          string     `json:"spec" bson:"spec"`
	State         RunState   `json:"state" bson:"state"`
	DueTime       time.Time  `json:"due_time" bson:"due_time"`
	StartTime     *time.Time `json:"start_time,omitempty" bson:"start_time,omitempty"`
	FinishTime    *time.Time `json:"finish_time,omitempty" bson:"finish_time,omitempty"`
	AutoScheduled bool       `json:"auto" bson:"auto"`
	Details       JobDetails `yaml:"-" json:"-" bson:"-"`
	Results       JobResults `yaml:"-" json:"-" bson:"-"`
	// DetailsStore is for storage and not used in code outside internal un/marshaling hooks, use `Details`.
	DetailsStore map[string]any `json:"details,omitempty" bson:"details,omitempty"`
	// ResultsStore is for storage and not used in code outside internal un/marshaling hooks, use `Results`.
	ResultsStore map[string]any `json:"results,omitempty" bson:"results,omitempty"`
	// this is DB-uniqued and its only purpose is to avoid two instances concurrently scheduling the same next job in `ensureJobSchedules`
	ScheduledNextAfterJob string `json:"prev,omitempty" bson:"prev,omitempty"`
	// FinalTaskFilter is obtained via call to Handler.TaskDetails() and stored for the later job finalization phase.
	FinalTaskFilter *TaskFilter `json:"task_filter,omitempty" bson:"task_filter,omitempty"`
	// FinalTaskListReq is obtained via call to Handler.TaskDetails() and stored for the later job finalization phase.
	FinalTaskListReq *ListRequest `json:"task_listreq,omitempty" bson:"task_listreq,omitempty"`

	Info struct { // Informational purposes only
		DurationPrepInMinutes     *float64           `json:"duration_prep_mins,omitempty" bson:"duration_prep_mins,omitempty"`
		DurationFinalizeInMinutes *float64           `json:"duration_finalize_mins,omitempty" bson:"duration_finalize_mins,omitempty"`
		CancellationReason        CancellationReason `json:"cancellation_reason,omitempty" bson:"cancellation_reason,omitempty"`
	} `json:"info,omitempty" bson:"info,omitempty"`

	ResourceVersion int `json:"resource_version" bson:"resource_version"`

	spec *JobSpec
}

func (it *Job) ctx(ctx context.Context, taskID string) *Context {
	return &Context{Context: ctx, Tenant: it.Tenant, JobID: it.ID, JobDetails: it.Details, JobSpec: *it.spec, TaskID: taskID}
}

type JobStats struct {
	TasksByState   map[RunState]int64 `json:"by_state"`
	TasksFailed    int64              `json:"num_failed"`
	TasksSucceeded int64              `json:"num_succeeded"`
	TasksTotal     int64              `json:"num_total"`

	DurationTotalMins    *float64 `json:"duration_total"`
	DurationPrepMins     *float64 `json:"duration_prep"`
	DurationFinalizeMins *float64 `json:"duration_finalize"`
}

// PercentDone returns a percentage `int` such that:
//   - 100 always means all tasks are DONE or CANCELLED,
//   - 0 always means no tasks are DONE or CANCELLED (or none exist yet),
//   - 1-99 means a (technically slightly imprecise) approximation of the actual ratio.
func (it *JobStats) PercentDone() int {
	switch it.TasksTotal {
	case 0, it.TasksByState[Pending] + it.TasksByState[Running]:
		return 0
	case it.TasksByState[Done] + it.TasksByState[Cancelled]:
		return 100
	default:
		return clamp(1, 99, int(float64(it.TasksByState[Done]+it.TasksByState[Cancelled])*(100.0/float64(it.TasksTotal))))
	}
}

// PercentSuccess returns a percentage `int` such that:
//   - 100 always means "job fully successful" (all its tasks succeeded),
//   - 0 always means "job fully failed" (all its tasks failed),
//   - 1-99 means a (technically slightly imprecise) approximation of the actual success/failure ratio,
//   - `nil` means the job is not yet `DONE`.
func (it *JobStats) PercentSuccess() *int {
	if it.TasksTotal == 0 || it.TasksByState[Done] != it.TasksTotal {
		return nil
	}
	switch it.TasksTotal {
	case it.TasksSucceeded:
		return ToPtr(100) // don't want 99 due to some 99.999999 float64 imprecision, neither would be want false positives (real 99.x% to mistakenly 100) from reckless `math.Ceil`ing...
	case it.TasksFailed:
		return ToPtr(0)
	default:
		return ToPtr(clamp(1, 99, int(float64(it.TasksSucceeded)*(100.0/float64(it.TasksTotal)))))
	}
}

type Task struct {
	Resource `json:",inline" bson:",inline"`

	HandlerID       string         `json:"job_type" bson:"job_type"`
	Job             string         `json:"job" bson:"job"`
	State           RunState       `json:"state,omitempty" bson:"state,omitempty"`
	StartTime       *time.Time     `json:"start_time,omitempty" bson:"start_time,omitempty"`
	FinishTime      *time.Time     `json:"finish_time,omitempty" bson:"finish_time,omitempty"`
	DetailsStore    map[string]any `json:"details,omitempty" bson:"details,omitempty"`
	ResultsStore    map[string]any `json:"results,omitempty" bson:"results,omitempty"`
	Attempts        []*TaskAttempt `json:"attempts" bson:"attempts"`
	ResourceVersion int            `json:"resource_version" bson:"resource_version"`

	job     *Job
	Details TaskDetails `yaml:"-" json:"-" bson:"-"`
	Results TaskResults `yaml:"-" json:"-" bson:"-"`
}

func (it *Task) Failed() bool {
	return it.State == Done && len(it.Attempts) > 0 && it.Attempts[0].Err != nil
}

func (it *Task) Succeeded() bool {
	return it.State == Done && len(it.Attempts) > 0 && it.Attempts[0].Err == nil
}

func (it *Task) JobSpec() string {
	if it.job == nil {
		return ""
	}
	return it.job.Spec
}

func (it *Task) markForRetryOrAsFailed(jobSpec *JobSpec) (retry bool) {
	if jobSpec != nil && len(it.Attempts) <= jobSpec.TaskRetries { // first attempt was not a RE-try
		it.State, it.StartTime, it.FinishTime = Pending, nil, nil
		return true
	}
	it.State, it.FinishTime = Done, timeNow()
	return false
}

type TaskAttempt struct {
	Time      time.Time  `json:"time,omitempty" bson:"time,omitempty"`
	TaskError *TaskError `json:"error,omitempty" bson:"error,omitempty"`

	// Err is the `error` equivalent of `TaskError`. For read accesses, both can be used interchangably. Write accesses (that last) don't anyway occur outside this package.
	Err error `json:"-" bson:"-"` // code in this package uses only `Err`, not `TaskError` which is just for storage and only used in un/marshaling hooks and API mapping code.
}

type TaskError struct {
	Message string `json:"message,omitempty" bson:"message,omitempty"`
}

func (it *TaskError) Err() error {
	if it == nil {
		return nil
	}
	return errors.New(it.Message)
}

func (it *TaskError) Error() (s string) {
	if it != nil {
		s = it.Err().Error()
	}
	return
}

// Timeout implements utils.HasTimeout
func (it *Task) Timeout() time.Duration {
	if it.job != nil && it.job.spec != nil && it.job.spec.Timeouts.TaskRun > 0 {
		return it.job.spec.Timeouts.TaskRun
	}
	return TimeoutLong
}

// Timeout implements utils.HasTimeout
func (it *Job) Timeout() time.Duration {
	if it.spec != nil && it.spec.Timeouts.JobPrepAndFinalize > 0 {
		return it.spec.Timeouts.JobPrepAndFinalize
	}
	return TimeoutLong
}

// everything below exists merely so that values of custom job-handler-owned types for the
// JobDetails / JobResults / TaskDetails / TaskResults are serialized/deserialized into/from the
// corresponding exported [Job|Task].[DetailsStore|ResultsStore] `map[string]any` fields from/into
// the actual Go-native custom struct types right when needed at bson/json marshal/unmarshal time.

func (it *Job) MarshalJSON() ([]byte, error)     { return it.marshal(json.Marshal) }
func (it *Job) UnmarshalJSON(data []byte) error  { return it.unmarshal(json.Unmarshal, data) }
func (it *Task) MarshalJSON() ([]byte, error)    { return it.marshal(json.Marshal) }
func (it *Task) UnmarshalJSON(data []byte) error { return it.unmarshal(json.Unmarshal, data) }

func (it *Job) marshal(marshaler func(any) ([]byte, error)) ([]byte, error) {
	type tmp Job // avoid eternal recursion
	it.onMarshaling()
	return onMarshal((*tmp)(it), marshaler, map[*map[string]any]handlerDefined{
		&it.DetailsStore: it.Details, &it.ResultsStore: it.Results})
}

func (it *Job) unmarshal(unmarshaler func([]byte, any) error, data []byte) error {
	type tmp Job // avoid eternal recursion
	return it.onUnmarshaled(
		onUnmarshal((*tmp)(it), it, unmarshaler, data, map[*map[string]any]*handlerDefined{
			&it.DetailsStore: &it.Details, &it.ResultsStore: &it.Results}))
}

func (it *Task) marshal(marshaler func(any) ([]byte, error)) ([]byte, error) {
	type tmp Task // avoid eternal recursion
	it.onMarshaling()
	return onMarshal((*tmp)(it), marshaler, map[*map[string]any]handlerDefined{
		&it.DetailsStore: it.Details, &it.ResultsStore: it.Results})
}

func (it *Task) unmarshal(unmarshaler func([]byte, any) error, data []byte) error {
	type tmp Task // avoid eternal recursion
	return it.onUnmarshaled(
		onUnmarshal((*tmp)(it), it, unmarshaler, data, map[*map[string]any]*handlerDefined{
			&it.DetailsStore: &it.Details, &it.ResultsStore: &it.Results}))
}

// when marshaling a Job/Task, both the DetailsStore & ResultsStore `map`s are filled from the `Details`/`Results` handler-specific live objects
func onMarshal(it any, marshaler func(any) ([]byte, error), ensure map[*map[string]any]handlerDefined) ([]byte, error) {
	for mapField, value := range ensure {
		*mapField = map[string]any{}
		if err := ensureMapFromValue(mapField, value); err != nil {
			return nil, err
		}
	}
	return marshaler(it)
}

// when unmarshaling a Job/Task, the `Details`/`Results` handler-specific objects must be filled from the DetailsStore/ResultsStore `map`s.
func onUnmarshal(itSafe any, itOrig interface {
	GetTenant() string
	handlerID() string
}, unmarshaler func([]byte, any) error, data []byte, ensure map[*map[string]any]*handlerDefined) error {
	err := unmarshaler(data, itSafe)
	if err != nil {
		return err
	}
	// only now after unmarshaling is `it.HandlerID` available
	if handler := handler(itOrig.handlerID()); handler != nil {
		switch jobOrTask := itOrig.(type) { // init new empty values to fill from maps
		case *Job:
			jobOrTask.Details, _ = handler.wellTypedJobDetails(nil)
			jobOrTask.Results, _ = handler.wellTypedJobResults(nil)
		case *Task:
			jobOrTask.Details, _ = handler.wellTypedTaskDetails(nil)
			jobOrTask.Results, _ = handler.wellTypedTaskResults(nil)
		}
		for mapField, value := range ensure {
			if err = ensureValueFromMap(mapField, value, itOrig.GetTenant()); err != nil {
				return err
			}
		}
	}
	return err
}

func (it *Job) handlerID() string  { return it.HandlerID }
func (it *Task) handlerID() string { return it.HandlerID }

type AsMap interface {
	FromMap(m map[string]any)
	ToMap() map[string]any
}

func ensureMapFromValue(mapField *map[string]any, value handlerDefined) (err error) {
	if value == nil {
		*mapField = nil
		return
	}
	if asMap, _ := value.(AsMap); asMap != nil {
		m := asMap.ToMap()
		delete(m, "tenant")
		*mapField = m
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, mapField); err == nil {
		delete(*mapField, "tenant")
	}
	return
}

func ensureValueFromMap(mapField *map[string]any, dst *handlerDefined, tenant string) error {
	if mapField == nil {
		*dst = nil
		return nil
	}
	m := *mapField
	if m == nil {
		*dst = nil
		return nil
	}
	m["tenant"] = tenant
	if asMap, _ := (*dst).(AsMap); asMap != nil {
		asMap.FromMap(m)
		return nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func (it *Job) onMarshaling() {
	ensureTZ(&it.DueTime, it.StartTime, it.FinishTime)
}

func (it *Job) onUnmarshaled(ret error) error {
	ensureTZ(&it.DueTime, it.StartTime, it.FinishTime)
	return ret
}

func (it *Task) onMarshaling() {
	ensureTZ(it.StartTime, it.FinishTime)
	for _, attempt := range it.Attempts {
		ensureTZ(&attempt.Time)
		if attempt.TaskError = nil; attempt.Err != nil {
			attempt.TaskError = &TaskError{Message: attempt.Err.Error()}
		}
	}
}

func (it *Task) onUnmarshaled(ret error) error {
	ensureTZ(it.StartTime, it.FinishTime)
	for _, attempt := range it.Attempts {
		ensureTZ(&attempt.Time)
		if attempt.Err = nil; attempt.TaskError != nil {
			attempt.Err = errors.New(attempt.TaskError.Message)
		}
	}
	return ret
}

func ensureTZ(times ...*time.Time) {
	for _, t := range times {
		if t != nil {
			*t = t.In(Timezone)
		}
	}
}
