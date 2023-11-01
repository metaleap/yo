# Jobs Scheduling & Execution **as-a-library**.

The lib and its data access patterns and state transition strategies are designed from the ground up for a multi-instance runtime reality.

## Packages

- `/pkg/engine` aka. the `jobs` package &mdash; the heart and work-horse logic of this job-scheduling-and-execution engine implementation.
- `/pkg/engine/backends/mongodb` &mdash; a MongoDB-based full implementation of the `jobs.Backend` interface that `engine` importers have to provide to `jobs.NewEngine`.
- `/pkg/engine/crontab` &mdash; self-contained cron expression parsing, plus calculation of soonest-upcoming &amp; most-recent point-in-time

## Data Model

- A `JobRun` is essentially a collection of `JobTask`s plus a `due_time`. (Alternatively call it "a single job run".)
    - `JobTask`s, not `JobRun`s, are the granular / atomic chunks of work that execute independently (potentially concurrently) and can "succeed" or "fail" or time-out or be-retried.
    - both data structures in `engine/job.go`
- A `JobDef` is like a template for `JobRun`s and declares common settings that apply to _all_ its `JobRun`s, such as eg. `timeouts`, `taskRetries`, `schedules` and more. (Alternatively call it "job definition" or "job template".)
    - Every newly-created (automatically or manually scheduled) `JobRun` names its "parent" `Def`.
    - data structure in `engine/jobdef.go`
- A `Handler` is all the actual custom logic of any one defific kind of job, and is set in the `JobDef`.
    - When a scheduled `JobRun` is at-or-past its `due_time` and is to actually run, its `Handler` (ie. your implementation) in this order:
        - prepares whatever preliminary/preparatory details / data / settings (that are common / shared among _all_ its tasks) will be needed (via `Handler.JobDetails`)
        - produces all the individual `JobTask`s that will belong to this particular `JobRun` (via `Handler.TaskDetails`)
        - runs the actual logic of a particular given `JobTask` (via `Handler.TaskResults`) when called to do so
        - finally at the end, gathers (if needed) any summary/aggregate outcome details/infos from the results of all the completed `JobTask`s (via `Handler.JobResults`)
    - interface defined &amp; documented in `engine/handler.go`
    - minimal example in `engine/handler_example.go`
- A `Backend` provides the complete storage-and-retrieval implementation that a `jobs.Engine` needs.
    - defined in `engine/backend.go`
    - full implementation example in `backends/mongodb`
- The `Engine` is what importers instantiate (`jobs.NewEngine`) and then start aka. `Resume()`.
    - exposes lifecycle-related methods: `CreateJob`, `CancelJob`, `DeleteJob`, `JobStats`.

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
    - the `Handler.JobDetails` are obtained
    - the `Handler.TaskDetails` are obtained
    - in one transaction:
        - all those just-prepared `JobTask`s are stored (in a `state` of `PENDING`)
        - the `Job.state` is set to `RUNNING`.
- While a job's `state` is `RUNNING`, the Engine keeps looking for `PENDING` tasks to run.
    - if there are none left, transition `JobRun` (in storage) into a `state` of `DONE`, at the same time storing its just-obtained `Handler.JobResults` (if any).
- When a `PENDING` task is picked up
    - first it is set to `RUNNING` in storage (to prevent multiple concurrent executions of the _actual_ work)
    - upon success, it is run via `Handler.TaskResults`
    - store outcome, whether error or results:
        - if timed out and retryable (as per JobDef settings), set to `PENDING` again
        - if errored but retryable (as per error kind and JobDef settings), set to `PENDING` again
        - else, set to `DONE`
    - if pod is restarted after starting but before completing that task run:
      - will be eventually detected and timed-out or marked-for-retry by another worker (`expireOrRetryDeadTasks`)
- When a `PENDING` or `RUNNING` job is `Cancel`ed:
    - the `JobRun` is transitioned (in storage) into a `state` of CANCELLING.
- While a `CANCELLING` job is found to exist:
    - find all `JobTask`s not yet `DONE` or `CANCELLED` (ie. `PENDING` or `RUNNING`)
        - if found: transition them (in storage) into a `state` of CANCELLED.
        - if none found: the `JobRun` itself is transitioned (in storage) into a `state` of CANCELLED.
