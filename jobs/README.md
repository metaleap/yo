# Jobs Scheduling & Execution **as-a-library**

The lib and its data access and state transition patterns are designed from the ground up for a multi-instance/single-DB runtime reality.

## Data Model

- A `JobRun` is essentially a collection of `JobTask`s plus a `due_time`.
    - `JobTask`s, not `JobRun`s, are the granular / atomic chunks of work that execute independently (potentially concurrently) and can "succeed" or "fail" or time-out or be-retried.
- A `JobDef` is like a template for `JobRun`s and declares common settings that apply to _all_ its `JobRun`s, such as eg. `timeouts`, `taskRetries`, `schedules` and more.
    - Every newly-created (automatically or manually scheduled) `JobRun` names its "parent" `JobDef`.
- A `JobType` is all the actual custom logic of any one specific kind of job, and is set in the `JobDef`.
    - When a scheduled `JobRun` is at-or-past its `due_time` and is to actually run, its `JobType` (ie. your implementation) in this order:
        - prepares whatever preliminary/preparatory details / data / settings (that are common / shared among _all_ its tasks) will be needed (via `JobType.JobDetails`)
        - produces all the individual `JobTask`s that will belong to this particular `JobRun` (via `JobType.TaskDetails`)
        - runs the actual logic of a particular given `JobTask` (via `JobType.TaskResults`) when called to do so
        - finally at the end, gathers (if needed) any summary/aggregate outcome details/infos from the results of all the completed `JobTask`s (via `JobType.JobResults`)
- The `Engine` is what importers instantiate (`NewEngine`) and then start aka. `Resume()`.
    - exposes lifecycle-related utility methods: `CreateJobRun`, `DeleteJobRun`, `Stats`.

## Lifecycle / State Transitions

- Every `JobRun`'s and `JobTask`'s `state` is always one of these `RunState`s:
    - PENDING
    - RUNNING
    - DONE
    - CANCELLING (`JobRun`s only, but not `JobTask`s)
    - CANCELLED

The lifecycle "state machinery" always operates as follows:

- Upon `JobRun` creation (whether automatically or manually scheduled), the job exists task-less by itself with its `due_time` and a `state` of `PENDING`.
- When a `PENDING` job is (over)due and to be started:
    - the `JobType.JobDetails` are obtained
    - the `JobType.TaskDetails` are obtained
    - in one transaction:
        - all those just-prepared `JobTask`s are stored (in a `state` of `PENDING`)
        - the `Job.state` is set to `RUNNING`.
- While a job's `state` is `RUNNING`, the Engine keeps looking for `PENDING` tasks to run.
    - if there are none left, transition `JobRun` (in storage) into a `state` of `DONE`, at the same time storing its just-obtained `JobType.JobResults` (if any).
- When a `PENDING` task is picked up
    - first it is set to `RUNNING` in storage (to prevent multiple concurrent executions of the _actual_ work)
    - upon success, it is run via `JobType.TaskResults`
    - store outcome, whether error or results:
        - if timed out and retryable (as per `JobDef` settings), set to `PENDING` again
        - if errored but retryable (as per `JobType.IsTaskErrRetryable` and `JobDef` settings), set to `PENDING` again
        - else, set to `DONE`
    - if pod is restarted after starting but before completing that task run:
      - will be eventually detected and timed-out or marked-for-retry by another worker
- When a `PENDING` or `RUNNING` job is `Cancel`ed:
    - the `JobRun` is transitioned (in storage) into a `state` of CANCELLING.
- While a `CANCELLING` job is found to exist:
    - find all `JobTask`s not yet `DONE` or `CANCELLED` (ie. `PENDING` or `RUNNING`)
        - if found: transition them (in storage) into a `state` of CANCELLED.
        - if none found: the `JobRun` itself is transitioned (in storage) into a `state` of CANCELLED.
