package jobs

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"

	. "yo/util"
	"yo/util/str"
)

// these 4 need to be type-aliases not type-decls due to our `onUnmarshal`

type JobDetails = handlerDefined
type JobResults = handlerDefined
type TaskDetails = handlerDefined
type TaskResults = handlerDefined

type handlerDefined interface {
	// nothing for now
}

// A `jobs.Handler` both creates the tasks for a particular scheduled job and runs them.
//   - An implementation must be highly stateless because the runs of the multiple tasks of any
//     given job are distributed among pods and run (or re-run) in no particular order.
//   - For all methods: expect a call to be repeated after some time (on this or another pod) if
//     the caller failed to store your results, or died beforehand (or upon returning `error`s).
//   - All `chan`s mentioned are never `close`d by your method impl, but always by its caller.
//   - JobDetails() -> initialize job
//   - TaskDetails() -> initialize all tasks for a job
//   - TaskResults() -> run the given task and return results
//   - JobResults() -> consume all task results and produce the job results
type Handler interface {
	// JobDetails is called when setting a `Job` from `PENDING` to `RUNNING`, just before TaskDetails.
	// Hence, JobDetails allows computing and storing shared preparatory details once that do not vary between tasks.
	// If the job was scheduled automatically, not manually, `ctx.JobDetails` is always `ctx.JobSpec.DefaultJobDetails` (which might be `nil`).
	// In the manual case, they may or may not be equal, depending on the `jobs.Create` call.
	// In either case only the *returned* `JobDetails` are stored (and later passed to the below methods).
	// Both `ctx.JobDetails` and the return value are of type *TJobDetails (that this `Handler` was `Register`ed with).
	JobDetails(ctx *Context) (JobDetails, error)

	// TaskDetails is called when setting a `Job` from `PENDING` to `RUNNING`.
	// This method prepares all the tasks for this job as `TaskDetails` and sends them to `stream`.
	// The caller `close`s `stream` right after this method returns.
	// The chan-of-slice design allows to send batches-of-multiples or one-by-one (depending
	// on whether you are paging through some data-set or similar considerations).
	// Each batch/slice sent equates to a DB save-multiple call (but all of them in 1 transaction).
	// If you send zero `TaskDetails`, your `TaskResults` won't ever be called, but `JobResults` will as usual (only with zero `Task`s in `stream`).
	// The `halt func(error) error` (which can be called with `nil`) has 2 purposes:
	//   - its return value, if not `nil`, indicates that you should stop sending
	//     and return, because the whole transaction is anyway aborted already.
	//   - if you pass it an error, this will also abort the whole transaction
	//     (in which case, also stop sending and return).
	// The `TaskDetails` you are sending are of type *TTaskDetails (that this `Handler` was `Register`ed with).
	// The `ctx.JobDetails` are of type *TJobDetails (that this `Handler` was `Register`ed with).
	// The return values (can be `nil`) will define sort+filter of the `Task`s stream passed to the final `JobResults` call.
	TaskDetails(ctx *Context, stream chan<- []TaskDetails, halt func(error) error) (*ListRequest, *TaskFilter)

	// TaskResults is called after a `Task` has been successfully set from `PENDING` to `RUNNING`.
	// It implements the actual execution of a Task previously prepared in this `Handler`'s `TaskDetails` method.
	// The `taskDetails` are of type *TTaskDetails (that this `Handler` was `Register`ed with).
	// The `ctx.JobDetails` are of type *TJobDetails (that this `Handler` was `Register`ed with).
	// The `TaskResults` returned are of type *TTaskResults (that this `Handler` was `Register`ed with).
	TaskResults(ctx *Context, taskDetails TaskDetails) (TaskResults, error)

	// JobResults is called when setting a job from `RUNNING` to `DONE`.
	// All `Task`s of the job are coming in over `stream()` (filtered+sorted as your above `TaskDetails()` method indicated).
	// (If `stream` is never called, its return `chan` is never created and its feeder DB query is not even performed. All calls to `stream()` return the exact same `chan`.)
	// For DONE `Task`s without `Results`, check its `Task.Attempts[0].Err` (`Task.Attempts` are sorted newest-to-oldest).
	// As soon as this method returns, `stream()` is `close`d by its caller.
	// Mutations to the `Task`s are ignored/discarded.
	// The `ctx.JobDetails` are of type *TJobDetails (that this `Handler` was `Register`ed with).
	// The `JobResults` returned are of type *TJobResults (that this `Handler` was `Register`ed with).
	// All `stream()[_].Details` are of type *TTaskDetails (that this `Handler` was `Register`ed with).
	// All `stream()[_].Results` are of type *TTaskResults (that this `Handler` was `Register`ed with).
	JobResults(ctx *Context, stream func() <-chan *Task) (JobResults, error)

	IsTaskErrRetryable(err error) bool
}

type Context struct {
	context.Context
	JobID      string
	JobDetails JobDetails
	JobSpec    JobSpec
	TaskID     string
}

type handlerReg struct {
	sync.Mutex
	new                  func(handlerID string) Handler
	wellTypedJobDetails  func(check handlerDefined) (new handlerDefined, err error)
	wellTypedJobResults  func(check handlerDefined) (new handlerDefined, err error)
	wellTypedTaskDetails func(check handlerDefined) (new handlerDefined, err error)
	wellTypedTaskResults func(check handlerDefined) (new handlerDefined, err error)
	byId                 map[string]Handler
}

var (
	registeredHandlers = map[string]*handlerReg{}
)

func Register[THandler Handler, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](newHandler func(handlerID string) THandler) error {
	return register[THandler, TJobDetails, TJobResults, TTaskDetails, TTaskResults](strings.TrimLeft(ReflType[THandler]().String(), "*"), newHandler)
}

func RegisterDefault[THandler Handler, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](newHandler func(handlerID string) THandler) {
	if err := register[THandler, TJobDetails, TJobResults, TTaskDetails, TTaskResults]("", newHandler); err != nil {
		panic(err)
	}
}

func register[THandler Handler, TJobDetails JobDetails, TJobResults JobResults, TTaskDetails TaskDetails, TTaskResults TaskResults](id string, newHandler func(handlerID string) THandler) error {
	if typeIs[TJobDetails](reflect.Pointer) || typeIs[TJobResults](reflect.Pointer) || typeIs[TTaskDetails](reflect.Pointer) || typeIs[TTaskResults](reflect.Pointer) {
		return errors.New("TJobDetails, TJobResults, TTaskDetails, TTaskResults must not be pointer types")
	}
	if !(typeIs[TJobDetails](reflect.Struct) && typeIs[TJobResults](reflect.Struct) && typeIs[TTaskDetails](reflect.Struct) && typeIs[TTaskResults](reflect.Struct)) {
		return errors.New("TJobDetails, TJobResults, TTaskDetails, TTaskResults must be `struct` types")
	}

	it := handlerReg{
		new:                  func(handlerID string) Handler { return newHandler(handlerID) },
		byId:                 map[string]Handler{},
		wellTypedJobDetails:  wellTypedFor[TJobDetails],
		wellTypedJobResults:  wellTypedFor[TJobResults],
		wellTypedTaskDetails: wellTypedFor[TTaskDetails],
		wellTypedTaskResults: wellTypedFor[TTaskResults],
	}
	if id != "" && registeredHandlers[id] != nil {
		return errors.New(str.Fmt("already have a `jobs.Handler` of type `%s` registered", id))
	}
	registeredHandlers[id] = &it
	return nil
}

func handler(id string) (ret *handlerReg) {
	if ret = registeredHandlers[id]; ret == nil {
		ret = registeredHandlers[""]
	}
	return
}

func wellTypedFor[TImpl handlerDefined](check handlerDefined) (handlerDefined, error) {
	if check != nil {
		if _, ok := check.(*TImpl); !ok {
			return nil, errors.New(str.Fmt("expected %s instead of %T", ReflType[*TImpl]().String(), check))
		}
	}
	return new(TImpl), nil
}

func (it *handlerReg) For(handlerID string) (ret Handler) {
	it.Lock()
	defer it.Unlock()
	if ret = it.byId[handlerID]; ret == nil {
		ret = it.new(handlerID)
		it.byId[handlerID] = ret
	}
	return
}

func typeIs[T any](kind reflect.Kind) bool {
	ty := ReflType[T]()
	return ty != nil && ty.Kind() == kind
}
