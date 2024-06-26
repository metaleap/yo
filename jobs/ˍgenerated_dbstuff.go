// Code generated by `yo/db/codegen_dbstuff.go`. DO NOT EDIT
package yojobs

import q "yo/db/query"

import sl "yo/util/sl"

func JobDefFields(fields ...JobDefField) []q.F { return sl.As(fields, JobDefField.F) }

type JobDefField q.F

const (
	JobDefId                               JobDefField = "Id"
	JobDefDtMade                           JobDefField = "DtMade"
	JobDefDtMod                            JobDefField = "DtMod"
	JobDefName                             JobDefField = "Name"
	JobDefJobTypeId                        JobDefField = "JobTypeId"
	JobDefDisabled                         JobDefField = "Disabled"
	JobDefAllowManualJobRuns               JobDefField = "AllowManualJobRuns"
	JobDefSchedules                        JobDefField = "Schedules"
	JobDefTimeoutSecsTaskRun               JobDefField = "TimeoutSecsTaskRun"
	JobDefTimeoutSecsJobRunPrepAndFinalize JobDefField = "TimeoutSecsJobRunPrepAndFinalize"
	JobDefMaxTaskRetries                   JobDefField = "MaxTaskRetries"
	JobDefDeleteAfterDays                  JobDefField = "DeleteAfterDays"
	JobDefRunTasklessJobs                  JobDefField = "RunTasklessJobs"
)

func (me JobDefField) ArrLen(a1 ...interface{}) q.Operand { return ((q.F)(me)).ArrLen(a1...) }
func (me JobDefField) Asc() q.OrderBy                     { return ((q.F)(me)).Asc() }
func (me JobDefField) Desc() q.OrderBy                    { return ((q.F)(me)).Desc() }
func (me JobDefField) Equal(a1 interface{}) q.Query       { return ((q.F)(me)).Equal(a1) }
func (me JobDefField) Eval(a1 interface{}, a2 func(q.C) q.F) interface{} {
	return ((q.F)(me)).Eval(a1, a2)
}
func (me JobDefField) F() q.F                                { return ((q.F)(me)).F() }
func (me JobDefField) GreaterOrEqual(a1 interface{}) q.Query { return ((q.F)(me)).GreaterOrEqual(a1) }
func (me JobDefField) GreaterThan(a1 interface{}) q.Query    { return ((q.F)(me)).GreaterThan(a1) }
func (me JobDefField) In(a1 ...interface{}) q.Query          { return ((q.F)(me)).In(a1...) }
func (me JobDefField) InArr(a1 interface{}) q.Query          { return ((q.F)(me)).InArr(a1) }
func (me JobDefField) LessOrEqual(a1 interface{}) q.Query    { return ((q.F)(me)).LessOrEqual(a1) }
func (me JobDefField) LessThan(a1 interface{}) q.Query       { return ((q.F)(me)).LessThan(a1) }
func (me JobDefField) Not() q.Query                          { return ((q.F)(me)).Not() }
func (me JobDefField) NotEqual(a1 interface{}) q.Query       { return ((q.F)(me)).NotEqual(a1) }
func (me JobDefField) NotIn(a1 ...interface{}) q.Query       { return ((q.F)(me)).NotIn(a1...) }
func (me JobDefField) NotInArr(a1 interface{}) q.Query       { return ((q.F)(me)).NotInArr(a1) }
func (me JobDefField) StrLen(a1 ...interface{}) q.Operand    { return ((q.F)(me)).StrLen(a1...) }

func JobRunFields(fields ...JobRunField) []q.F { return sl.As(fields, JobRunField.F) }

type JobRunField q.F

const (
	JobRunId                                      JobRunField = "Id"
	JobRunDtMade                                  JobRunField = "DtMade"
	JobRunDtMod                                   JobRunField = "DtMod"
	JobRunVersion                                 JobRunField = "Version"
	JobRunJobTypeId                               JobRunField = "JobTypeId"
	JobRunJobDef                                  JobRunField = "JobDef"
	jobRunState                                   JobRunField = "state"
	JobRunCancelReason                            JobRunField = "CancelReason"
	JobRunDueTime                                 JobRunField = "DueTime"
	JobRunStartTime                               JobRunField = "StartTime"
	JobRunFinishTime                              JobRunField = "FinishTime"
	JobRunAutoScheduled                           JobRunField = "AutoScheduled"
	JobRunScheduledNextAfter                      JobRunField = "ScheduledNextAfter"
	JobRunDurationPrepSecs                        JobRunField = "DurationPrepSecs"
	JobRunDurationFinalizeSecs                    JobRunField = "DurationFinalizeSecs"
	jobRunDetails                                 JobRunField = "details"
	jobRunResults                                 JobRunField = "results"
	JobRunJobDef_Id                               JobRunField = "JobDef.Id"
	JobRunJobDef_DtMade                           JobRunField = "JobDef.DtMade"
	JobRunJobDef_DtMod                            JobRunField = "JobDef.DtMod"
	JobRunJobDef_Name                             JobRunField = "JobDef.Name"
	JobRunJobDef_JobTypeId                        JobRunField = "JobDef.JobTypeId"
	JobRunJobDef_Disabled                         JobRunField = "JobDef.Disabled"
	JobRunJobDef_AllowManualJobRuns               JobRunField = "JobDef.AllowManualJobRuns"
	JobRunJobDef_Schedules                        JobRunField = "JobDef.Schedules"
	JobRunJobDef_TimeoutSecsTaskRun               JobRunField = "JobDef.TimeoutSecsTaskRun"
	JobRunJobDef_TimeoutSecsJobRunPrepAndFinalize JobRunField = "JobDef.TimeoutSecsJobRunPrepAndFinalize"
	JobRunJobDef_MaxTaskRetries                   JobRunField = "JobDef.MaxTaskRetries"
	JobRunJobDef_DeleteAfterDays                  JobRunField = "JobDef.DeleteAfterDays"
	JobRunJobDef_RunTasklessJobs                  JobRunField = "JobDef.RunTasklessJobs"
	JobRunScheduledNextAfter_Id                   JobRunField = "ScheduledNextAfter.Id"
	JobRunScheduledNextAfter_DtMade               JobRunField = "ScheduledNextAfter.DtMade"
	JobRunScheduledNextAfter_DtMod                JobRunField = "ScheduledNextAfter.DtMod"
	JobRunScheduledNextAfter_Version              JobRunField = "ScheduledNextAfter.Version"
	JobRunScheduledNextAfter_JobTypeId            JobRunField = "ScheduledNextAfter.JobTypeId"
	JobRunScheduledNextAfter_JobDef               JobRunField = "ScheduledNextAfter.JobDef"
	jobRunScheduledNextAfter_state                JobRunField = "ScheduledNextAfter.state"
	JobRunScheduledNextAfter_CancelReason         JobRunField = "ScheduledNextAfter.CancelReason"
	JobRunScheduledNextAfter_DueTime              JobRunField = "ScheduledNextAfter.DueTime"
	JobRunScheduledNextAfter_StartTime            JobRunField = "ScheduledNextAfter.StartTime"
	JobRunScheduledNextAfter_FinishTime           JobRunField = "ScheduledNextAfter.FinishTime"
	JobRunScheduledNextAfter_AutoScheduled        JobRunField = "ScheduledNextAfter.AutoScheduled"
	JobRunScheduledNextAfter_ScheduledNextAfter   JobRunField = "ScheduledNextAfter.ScheduledNextAfter"
	JobRunScheduledNextAfter_DurationPrepSecs     JobRunField = "ScheduledNextAfter.DurationPrepSecs"
	JobRunScheduledNextAfter_DurationFinalizeSecs JobRunField = "ScheduledNextAfter.DurationFinalizeSecs"
	jobRunScheduledNextAfter_details              JobRunField = "ScheduledNextAfter.details"
	jobRunScheduledNextAfter_results              JobRunField = "ScheduledNextAfter.results"
)

func (me JobRunField) ArrLen(a1 ...interface{}) q.Operand { return ((q.F)(me)).ArrLen(a1...) }
func (me JobRunField) Asc() q.OrderBy                     { return ((q.F)(me)).Asc() }
func (me JobRunField) Desc() q.OrderBy                    { return ((q.F)(me)).Desc() }
func (me JobRunField) Equal(a1 interface{}) q.Query       { return ((q.F)(me)).Equal(a1) }
func (me JobRunField) Eval(a1 interface{}, a2 func(q.C) q.F) interface{} {
	return ((q.F)(me)).Eval(a1, a2)
}
func (me JobRunField) F() q.F                                { return ((q.F)(me)).F() }
func (me JobRunField) GreaterOrEqual(a1 interface{}) q.Query { return ((q.F)(me)).GreaterOrEqual(a1) }
func (me JobRunField) GreaterThan(a1 interface{}) q.Query    { return ((q.F)(me)).GreaterThan(a1) }
func (me JobRunField) In(a1 ...interface{}) q.Query          { return ((q.F)(me)).In(a1...) }
func (me JobRunField) InArr(a1 interface{}) q.Query          { return ((q.F)(me)).InArr(a1) }
func (me JobRunField) LessOrEqual(a1 interface{}) q.Query    { return ((q.F)(me)).LessOrEqual(a1) }
func (me JobRunField) LessThan(a1 interface{}) q.Query       { return ((q.F)(me)).LessThan(a1) }
func (me JobRunField) Not() q.Query                          { return ((q.F)(me)).Not() }
func (me JobRunField) NotEqual(a1 interface{}) q.Query       { return ((q.F)(me)).NotEqual(a1) }
func (me JobRunField) NotIn(a1 ...interface{}) q.Query       { return ((q.F)(me)).NotIn(a1...) }
func (me JobRunField) NotInArr(a1 interface{}) q.Query       { return ((q.F)(me)).NotInArr(a1) }
func (me JobRunField) StrLen(a1 ...interface{}) q.Operand    { return ((q.F)(me)).StrLen(a1...) }

func JobTaskFields(fields ...JobTaskField) []q.F { return sl.As(fields, JobTaskField.F) }

type JobTaskField q.F

const (
	JobTaskId                          JobTaskField = "Id"
	JobTaskDtMade                      JobTaskField = "DtMade"
	JobTaskDtMod                       JobTaskField = "DtMod"
	JobTaskVersion                     JobTaskField = "Version"
	JobTaskJobTypeId                   JobTaskField = "JobTypeId"
	JobTaskJobRun                      JobTaskField = "JobRun"
	jobTaskState                       JobTaskField = "state"
	JobTaskStartTime                   JobTaskField = "StartTime"
	JobTaskFinishTime                  JobTaskField = "FinishTime"
	JobTaskAttempts                    JobTaskField = "Attempts"
	jobTaskDetails                     JobTaskField = "details"
	jobTaskResults                     JobTaskField = "results"
	JobTaskJobRun_Id                   JobTaskField = "JobRun.Id"
	JobTaskJobRun_DtMade               JobTaskField = "JobRun.DtMade"
	JobTaskJobRun_DtMod                JobTaskField = "JobRun.DtMod"
	JobTaskJobRun_Version              JobTaskField = "JobRun.Version"
	JobTaskJobRun_JobTypeId            JobTaskField = "JobRun.JobTypeId"
	JobTaskJobRun_JobDef               JobTaskField = "JobRun.JobDef"
	jobTaskJobRun_state                JobTaskField = "JobRun.state"
	JobTaskJobRun_CancelReason         JobTaskField = "JobRun.CancelReason"
	JobTaskJobRun_DueTime              JobTaskField = "JobRun.DueTime"
	JobTaskJobRun_StartTime            JobTaskField = "JobRun.StartTime"
	JobTaskJobRun_FinishTime           JobTaskField = "JobRun.FinishTime"
	JobTaskJobRun_AutoScheduled        JobTaskField = "JobRun.AutoScheduled"
	JobTaskJobRun_ScheduledNextAfter   JobTaskField = "JobRun.ScheduledNextAfter"
	JobTaskJobRun_DurationPrepSecs     JobTaskField = "JobRun.DurationPrepSecs"
	JobTaskJobRun_DurationFinalizeSecs JobTaskField = "JobRun.DurationFinalizeSecs"
	jobTaskJobRun_details              JobTaskField = "JobRun.details"
	jobTaskJobRun_results              JobTaskField = "JobRun.results"
)

func (me JobTaskField) ArrLen(a1 ...interface{}) q.Operand { return ((q.F)(me)).ArrLen(a1...) }
func (me JobTaskField) Asc() q.OrderBy                     { return ((q.F)(me)).Asc() }
func (me JobTaskField) Desc() q.OrderBy                    { return ((q.F)(me)).Desc() }
func (me JobTaskField) Equal(a1 interface{}) q.Query       { return ((q.F)(me)).Equal(a1) }
func (me JobTaskField) Eval(a1 interface{}, a2 func(q.C) q.F) interface{} {
	return ((q.F)(me)).Eval(a1, a2)
}
func (me JobTaskField) F() q.F                                { return ((q.F)(me)).F() }
func (me JobTaskField) GreaterOrEqual(a1 interface{}) q.Query { return ((q.F)(me)).GreaterOrEqual(a1) }
func (me JobTaskField) GreaterThan(a1 interface{}) q.Query    { return ((q.F)(me)).GreaterThan(a1) }
func (me JobTaskField) In(a1 ...interface{}) q.Query          { return ((q.F)(me)).In(a1...) }
func (me JobTaskField) InArr(a1 interface{}) q.Query          { return ((q.F)(me)).InArr(a1) }
func (me JobTaskField) LessOrEqual(a1 interface{}) q.Query    { return ((q.F)(me)).LessOrEqual(a1) }
func (me JobTaskField) LessThan(a1 interface{}) q.Query       { return ((q.F)(me)).LessThan(a1) }
func (me JobTaskField) Not() q.Query                          { return ((q.F)(me)).Not() }
func (me JobTaskField) NotEqual(a1 interface{}) q.Query       { return ((q.F)(me)).NotEqual(a1) }
func (me JobTaskField) NotIn(a1 ...interface{}) q.Query       { return ((q.F)(me)).NotIn(a1...) }
func (me JobTaskField) NotInArr(a1 interface{}) q.Query       { return ((q.F)(me)).NotInArr(a1) }
func (me JobTaskField) StrLen(a1 ...interface{}) q.Operand    { return ((q.F)(me)).StrLen(a1...) }
