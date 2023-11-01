package jobs

import (
	"encoding/json"
	"errors"
)

// everything below exists merely so that values of custom JobType-owned types for the
// JobDetails / JobResults / TaskDetails / TaskResults are serialized/deserialized into/from the
// corresponding exported [Job|Task].[DetailsStore|ResultsStore] `map[string]any` fields from/into
// the actual Go-native custom struct types right when needed at json marshal/unmarshal time.

func (it *JobRun) MarshalJSON() ([]byte, error)    { return it.marshal(json.Marshal) }
func (it *JobRun) UnmarshalJSON(data []byte) error { return it.unmarshal(json.Unmarshal, data) }
func (it *Task) MarshalJSON() ([]byte, error)      { return it.marshal(json.Marshal) }
func (it *Task) UnmarshalJSON(data []byte) error   { return it.unmarshal(json.Unmarshal, data) }

func (it *JobRun) marshal(marshaler func(any) ([]byte, error)) ([]byte, error) {
	type tmp JobRun // avoid eternal recursion
	it.onMarshaling()
	return onMarshal((*tmp)(it), marshaler, map[*map[string]any]jobTypeDefined{
		&it.DetailsStore: it.Details, &it.ResultsStore: it.Results})
}

func (it *JobRun) unmarshal(unmarshaler func([]byte, any) error, data []byte) error {
	type tmp JobRun // avoid eternal recursion
	return it.onUnmarshaled(
		onUnmarshal((*tmp)(it), it, unmarshaler, data, map[*map[string]any]*jobTypeDefined{
			&it.DetailsStore: &it.Details, &it.ResultsStore: &it.Results}))
}

func (it *Task) marshal(marshaler func(any) ([]byte, error)) ([]byte, error) {
	type tmp Task // avoid eternal recursion
	it.onMarshaling()
	return onMarshal((*tmp)(it), marshaler, map[*map[string]any]jobTypeDefined{
		&it.DetailsStore: it.Details, &it.ResultsStore: it.Results})
}

func (it *Task) unmarshal(unmarshaler func([]byte, any) error, data []byte) error {
	type tmp Task // avoid eternal recursion
	return it.onUnmarshaled(
		onUnmarshal((*tmp)(it), it, unmarshaler, data, map[*map[string]any]*jobTypeDefined{
			&it.DetailsStore: &it.Details, &it.ResultsStore: &it.Results}))
}

// when marshaling a Job/Task, both the DetailsStore & ResultsStore `map`s are filled from the `Details`/`Results` JobType-specific live objects
func onMarshal(it any, marshaler func(any) ([]byte, error), ensure map[*map[string]any]jobTypeDefined) ([]byte, error) {
	for mapField, value := range ensure {
		*mapField = map[string]any{}
		if err := ensureMapFromValue(mapField, value); err != nil {
			return nil, err
		}
	}
	return marshaler(it)
}

// when unmarshaling a Job/Task, the `Details`/`Results` JobType-specific objects must be filled from the DetailsStore/ResultsStore `map`s.
func onUnmarshal(itSafe any, itOrig interface {
	jobTypeId() string
}, unmarshaler func([]byte, any) error, data []byte, ensure map[*map[string]any]*jobTypeDefined) error {
	err := unmarshaler(data, itSafe)
	if err != nil {
		return err
	}
	// only now after unmarshaling is `it.JobTypeId` available
	if job_type := jobType(itOrig.jobTypeId()); job_type != nil {
		switch jobOrTask := itOrig.(type) { // init new empty values to fill from maps
		case *JobRun:
			jobOrTask.Details, _ = job_type.wellTypedJobDetails(nil)
			jobOrTask.Results, _ = job_type.wellTypedJobResults(nil)
		case *Task:
			jobOrTask.Details, _ = job_type.wellTypedTaskDetails(nil)
			jobOrTask.Results, _ = job_type.wellTypedTaskResults(nil)
		}
		for mapField, value := range ensure {
			if err = ensureValueFromMap(mapField, value); err != nil {
				return err
			}
		}
	}
	return err
}

func (it *JobRun) jobTypeId() string { return it.JobTypeId }
func (it *Task) jobTypeId() string   { return it.JobTypeId }

type AsMap interface {
	FromMap(m map[string]any)
	ToMap() map[string]any
}

func ensureMapFromValue(mapField *map[string]any, value jobTypeDefined) error {
	if value == nil {
		*mapField = nil
		return nil
	} else if asMap, _ := value.(AsMap); asMap != nil {
		*mapField = asMap.ToMap()
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, mapField)
}

func ensureValueFromMap(mapField *map[string]any, dst *jobTypeDefined) error {
	if mapField == nil {
		*dst = nil
		return nil
	}
	m := *mapField
	if m == nil {
		*dst = nil
		return nil
	}
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

func (it *JobRun) onMarshaling() {
	ensureTZ(&it.DueTime, it.StartTime, it.FinishTime)
}

func (it *JobRun) onUnmarshaled(ret error) error {
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
