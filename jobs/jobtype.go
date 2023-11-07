package yojobs

import (
	"reflect"
	"strings"
	"sync"

	. "yo/ctx"
	yodb "yo/db"
	yojson "yo/json"
	. "yo/util"
	"yo/util/str"
)

// these 4 need to be type-aliases not type-decls due to our `onUnmarshal`

type JobDetails = jobTypeDefined
type JobResults = jobTypeDefined
type TaskDetails = jobTypeDefined
type TaskResults = jobTypeDefined

type jobTypeDefined interface {
	// nothing for now
}

// A `JobType` both creates the tasks for a particular scheduled job and runs them.
//   - An implementation must be highly stateless because the runs of the multiple tasks of any
//     given job are distributed among pods and run (or re-run) in no particular order.
//   - For all methods: expect a call to be repeated after some time (on this or another pod) if
//     the caller failed to store your results, or died beforehand (or upon returning `error`s).
//   - JobDetails() -> initialize job
//   - TaskDetails() -> initialize all tasks for a job
//   - TaskResults() -> run the given task and return results
//   - JobResults() -> consume all task results and produce the job results
type JobType interface {
	// JobDetails is called when setting a `JobRun` from `PENDING` to `RUNNING`, just before `TaskDetails`.
	// Hence, JobDetails allows computing and storing shared preparatory details once that do not vary between tasks.
	// If the job was scheduled automatically, not manually, `ctx.Job.Details` is always `nil`.
	// In the manual case, they may or may not be equal, depending on the `CreateJobRun` call.
	// In either case only the *returned* `JobDetails` are stored (and later passed to the below methods).
	// Both `ctx.Job.Details` and the return value are of type *TJobDetails (that this `JobType` was `Register`ed with).
	JobDetails(ctx *Ctx) JobDetails

	// TaskDetails is called when setting a `JobRun` from `PENDING` to `RUNNING`.
	// This method prepares all the tasks for this job as `TaskDetails` and sends them over `stream`.
	// Any calls to `stream` right after this method returns will panic.
	// The `stream` func param allows to send batches-of-multiples or one-by-one (depending
	// on whether you are paging through some data-set or similar considerations).
	// Each batch/slice sent equates to a DB save-multiple call (but all of them in 1 transaction).
	// The `TaskDetails` you are sending are of type *TTaskDetails (that this `JobType` was `Register`ed with).
	// The `ctx.Job.Details` are of type *TJobDetails (that this `JobType` was `Register`ed with).
	TaskDetails(ctx *Ctx, stream func([]TaskDetails))

	// TaskResults is called after a `JobTask` has been successfully set from `PENDING` to `RUNNING`.
	// It implements the actual execution of a Task previously prepared in this `JobType`'s `TaskDetails` method.
	// The `taskDetails` are of type *TTaskDetails (that this `JobType` was `Register`ed with).
	// The `ctx.Job.Details` are of type *TJobDetails (that this `JobType` was `Register`ed with).
	// The `TaskResults` returned are of type *TTaskResults (that this `JobType` was `Register`ed with).
	TaskResults(ctx *Ctx, taskDetails TaskDetails) TaskResults

	// JobResults is called when setting a job from `RUNNING` to `DONE`.
	// All `JobTask`s of the job are coming in over `stream()`, which can be `nil` if none are needed.
	// The final `results()` producer is called (unless `nil`) after the last call to `stream`.
	// For DONE `JobTask`s without `Results`, check its `Task.Attempts[0].Err` (`Task.Attempts` are sorted newest-to-oldest).
	// Mutations to the `JobTask`s are ignored/discarded.
	// The `ctx.Job.Details` are of type *TJobDetails (that this `JobType` was `Register`ed with).
	// The `results()` returned are of type *TJobResults (that this `JobType` was `Register`ed with).
	// All `stream().Details` are of type *TTaskDetails (that this `JobType` was `Register`ed with).
	// All `stream().Results` are of type *TTaskResults (that this `JobType` was `Register`ed with).
	// The `stream` must not use `ctx` for any DB operations. Instead, it should obtain a Ctx from its first argument.
	JobResults(ctx *Ctx) (stream func(func() *Ctx, *JobTask, *bool), results func() JobResults)
}

type jobTypeReg struct {
	sync.Mutex
	new                  func(jobTypeId string) JobType
	checkTypeJobDetails  func(check jobTypeDefined)
	checkTypeJobResults  func(check jobTypeDefined)
	checkTypeTaskDetails func(check jobTypeDefined)
	checkTypeTaskResults func(check jobTypeDefined)
	loadJobDetails       func(yodb.JsonMap[any]) jobTypeDefined
	loadJobResults       func(yodb.JsonMap[any]) jobTypeDefined
	loadTaskDetails      func(yodb.JsonMap[any]) jobTypeDefined
	loadTaskResults      func(yodb.JsonMap[any]) jobTypeDefined
	byId                 map[string]JobType
}

var registeredJobTypes = map[string]*jobTypeReg{}

func Register[TJobType JobType, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](new func(string) TJobType) string {
	id := strings.TrimLeft(ReflType[TJobType]().String(), "*")
	register[TJobType, TJobDetails, TJobResults, TTaskDetails, TTaskResults](id, new)
	return id
}

func RegisterDefault[TJobType JobType, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](new func(string) TJobType) {
	register[TJobType, TJobDetails, TJobResults, TTaskDetails, TTaskResults]("", new)
}

func register[TJobType JobType, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](id string, new func(string) TJobType) string {
	if typeIs[TJobDetails](reflect.Pointer) || typeIs[TJobResults](reflect.Pointer) || typeIs[TTaskDetails](reflect.Pointer) || typeIs[TTaskResults](reflect.Pointer) {
		panic("TJobDetails, TJobResults, TTaskDetails, TTaskResults must not be pointer types")
	}
	if !(typeIs[TJobDetails](reflect.Struct) && typeIs[TJobResults](reflect.Struct) && typeIs[TTaskDetails](reflect.Struct) && typeIs[TTaskResults](reflect.Struct)) {
		panic("TJobDetails, TJobResults, TTaskDetails, TTaskResults must be `struct` types")
	}

	it := jobTypeReg{
		new:                  func(jobTypeId string) JobType { return new(jobTypeId) },
		byId:                 map[string]JobType{},
		checkTypeJobDetails:  jobTypeCheckType[TJobDetails],
		checkTypeJobResults:  jobTypeCheckType[TJobResults],
		checkTypeTaskDetails: jobTypeCheckType[TTaskDetails],
		checkTypeTaskResults: jobTypeCheckType[TTaskResults],
		loadJobDetails:       jobTypeLoadFromDict[TJobDetails],
		loadJobResults:       jobTypeLoadFromDict[TJobResults],
		loadTaskDetails:      jobTypeLoadFromDict[TTaskDetails],
		loadTaskResults:      jobTypeLoadFromDict[TTaskResults],
	}
	if id != "" && registeredJobTypes[id] != nil {
		panic(str.Fmt("already have a `JobType` of type `%s` registered", id))
	}
	if IsDevMode {
		println("RegJobType:'" + id + "'")
	}
	registeredJobTypes[id] = &it
	return id
}

func jobType(id string) (ret *jobTypeReg) {
	if ret = registeredJobTypes[id]; ret == nil {
		ret = registeredJobTypes[""]
	}
	return
}

func JobTypeExists(id string) (ret bool) {
	_, ret = registeredJobTypes[id]
	return
}

func jobTypeCheckType[TImpl jobTypeDefined](objToCheck jobTypeDefined) {
	if IsDevMode {
		if _, ok := objToCheck.(*TImpl); (!ok) && (objToCheck != nil) {
			panic(str.Fmt("expected %s instead of %T", ReflType[*TImpl]().String(), objToCheck))
		}
	}
}

func jobTypeLoadFromDict[TImpl jobTypeDefined](fromDict yodb.JsonMap[any]) jobTypeDefined {
	ret := yojson.FromDict[TImpl](fromDict)
	return &ret
}

func (me *jobTypeReg) ById(jobTypeId string) (ret JobType) {
	me.Lock()
	defer me.Unlock()
	if ret = me.byId[jobTypeId]; ret == nil {
		ret = me.new(jobTypeId)
		me.byId[jobTypeId] = ret
	}
	return
}

func typeIs[T any](kind reflect.Kind) bool {
	ty := ReflType[T]()
	return (ty != nil) && (ty.Kind() == kind)
}
