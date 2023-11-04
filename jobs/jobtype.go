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
//   - All `chan`s mentioned are never `close`d by your method impl, but always by its caller.
//   - JobDetails() -> initialize job
//   - TaskDetails() -> initialize all tasks for a job
//   - TaskResults() -> run the given task and return results
//   - JobResults() -> consume all task results and produce the job results
type JobType interface {
	// JobDetails is called when setting a `JobRun` from `PENDING` to `RUNNING`, just before `TaskDetails`.
	// Hence, JobDetails allows computing and storing shared preparatory details once that do not vary between tasks.
	// If the job was scheduled automatically, not manually, `ctx.JobDetails` is always `ctx.JobDef.DefaultJobDetails` (which might be `nil`).
	// In the manual case, they may or may not be equal, depending on the `CreateJobRun` call.
	// In either case only the *returned* `JobDetails` are stored (and later passed to the below methods).
	// Both `ctx.JobDetails` and the return value are of type *TJobDetails (that this `JobType` was `Register`ed with).
	JobDetails(ctx *Context) JobDetails

	// TaskDetails is called when setting a `JobRun` from `PENDING` to `RUNNING`.
	// This method prepares all the tasks for this job as `TaskDetails` and sends them to `stream`.
	// The caller `close`s `stream` right after this method returns.
	// The chan-of-slice design allows to send batches-of-multiples or one-by-one (depending
	// on whether you are paging through some data-set or similar considerations).
	// Each batch/slice sent equates to a DB save-multiple call (but all of them in 1 transaction).
	// If you send zero `TaskDetails`, your `TaskResults` won't ever be called, but `JobResults` will as usual (only with zero `JobTask`s in `stream`).
	// The `TaskDetails` you are sending are of type *TTaskDetails (that this `JobType` was `Register`ed with).
	// The `ctx.JobDetails` are of type *TJobDetails (that this `JobType` was `Register`ed with).
	TaskDetails(ctx *Context, stream chan<- []TaskDetails)

	// TaskResults is called after a `JobTask` has been successfully set from `PENDING` to `RUNNING`.
	// It implements the actual execution of a Task previously prepared in this `JobType`'s `TaskDetails` method.
	// The `taskDetails` are of type *TTaskDetails (that this `JobType` was `Register`ed with).
	// The `ctx.JobDetails` are of type *TJobDetails (that this `JobType` was `Register`ed with).
	// The `TaskResults` returned are of type *TTaskResults (that this `JobType` was `Register`ed with).
	TaskResults(ctx *Context, taskDetails TaskDetails) TaskResults

	// JobResults is called when setting a job from `RUNNING` to `DONE`.
	// All `JobTask`s of the job are coming in over `stream()` (filtered+sorted as your above `TaskDetails()` method indicated).
	// (If `stream` is never called, its return `chan` is never created and its feeder DB query is not even performed. All calls to `stream()` return the exact same `chan`.)
	// For DONE `JobTask`s without `Results`, check its `Task.Attempts[0].Err` (`Task.Attempts` are sorted newest-to-oldest).
	// As soon as this method returns, `stream()` is `close`d by its caller.
	// Mutations to the `JobTask`s are ignored/discarded.
	// The `ctx.JobDetails` are of type *TJobDetails (that this `JobType` was `Register`ed with).
	// The `JobResults` returned are of type *TJobResults (that this `JobType` was `Register`ed with).
	// All `stream()[_].Details` are of type *TTaskDetails (that this `JobType` was `Register`ed with).
	// All `stream()[_].Results` are of type *TTaskResults (that this `JobType` was `Register`ed with).
	JobResults(ctx *Context, stream func() <-chan *JobTask) JobResults
}

type Context struct {
	*Ctx
	JobRunId   yodb.I64
	JobDetails JobDetails
	JobDef     JobDef
	JobTaskId  yodb.I64
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

func Register[TJobType JobType, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](new func(string) TJobType) {
	register[TJobType, TJobDetails, TJobResults, TTaskDetails, TTaskResults](strings.TrimLeft(ReflType[TJobType]().String(), "*"), new)
}

func RegisterDefault[TJobType JobType, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](new func(string) TJobType) {
	register[TJobType, TJobDetails, TJobResults, TTaskDetails, TTaskResults]("", new)
}

func register[TJobType JobType, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](id string, new func(string) TJobType) {
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
}

func jobType(id string) (ret *jobTypeReg) {
	if ret = registeredJobTypes[id]; ret == nil {
		ret = registeredJobTypes[""]
	}
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
