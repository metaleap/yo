// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT
package yodb

import reflect "reflect"
import yosrv "yo/srv"
import util "yo/util"
import q "yo/db/query"

type _ = q.F // just in case of no other generated import users
type apiPkgInfo util.Void

func (apiPkgInfo) PkgName() string    { return "yodb" }
func (me apiPkgInfo) PkgPath() string { return reflect.TypeOf(me).PkgPath() }

var yodbPkg = apiPkgInfo{}

func api[TIn any, TOut any](f func(*yosrv.ApiCtx[TIn, TOut]), failIfs ...yosrv.Fails) yosrv.ApiMethod {
	return yosrv.Api[TIn, TOut](f, failIfs...).From(yodbPkg)
}

const ErrQuery_ExpectedOneOrNoneButNotMultipleOfFldOrStrOrBoolOrInt util.Err = "Query_ExpectedOneOrNoneButNotMultipleOfFldOrStrOrBoolOrInt"
const ErrQuery_ExpectedOnlyEitherQueryOrQueryFromButNotBoth util.Err = "Query_ExpectedOnlyEitherQueryOrQueryFromButNotBoth"
const ErrQuery_ExpectedSetOperandForIN util.Err = "Query_ExpectedSetOperandForIN"
const ErrQuery_ExpectedTwoOperandsForAND util.Err = "Query_ExpectedTwoOperandsForAND"
const ErrQuery_ExpectedTwoOperandsForEQ util.Err = "Query_ExpectedTwoOperandsForEQ"
const ErrQuery_ExpectedTwoOperandsForGE util.Err = "Query_ExpectedTwoOperandsForGE"
const ErrQuery_ExpectedTwoOperandsForGT util.Err = "Query_ExpectedTwoOperandsForGT"
const ErrQuery_ExpectedTwoOperandsForIN util.Err = "Query_ExpectedTwoOperandsForIN"
const ErrQuery_ExpectedTwoOperandsForLE util.Err = "Query_ExpectedTwoOperandsForLE"
const ErrQuery_ExpectedTwoOperandsForLT util.Err = "Query_ExpectedTwoOperandsForLT"
const ErrQuery_ExpectedTwoOperandsForNE util.Err = "Query_ExpectedTwoOperandsForNE"
const ErrQuery_ExpectedTwoOperandsForNOT util.Err = "Query_ExpectedTwoOperandsForNOT"
const ErrQuery_ExpectedTwoOperandsForOR util.Err = "Query_ExpectedTwoOperandsForOR"
const ErrDbDelete_ExpectedQueryForDelete util.Err = "DbDelete_ExpectedQueryForDelete"
const ErrDbUpdate_ExpectedChangesForUpdate util.Err = "DbUpdate_ExpectedChangesForUpdate"
const ErrDbUpdate_ExpectedQueryForUpdate util.Err = "DbUpdate_ExpectedQueryForUpdate"
const ___yo_db_ErrEntry_countMax = q.F("Max")
const ___yo_db_ErrEntry_countOrderBy = q.F("OrderBy")
const ___yo_db_ErrEntry_countQuery = q.F("Query")
const ___yo_db_ErrEntry_countQueryFrom = q.F("QueryFrom")
const ___yo_db_ErrEntry_createManyItems = q.F("Items")
const ___yo_db_ErrEntry_createOneCtxVals = q.F("CtxVals")
const ___yo_db_ErrEntry_createOneDtMade = q.F("DtMade")
const ___yo_db_ErrEntry_createOneDtMod = q.F("DtMod")
const ___yo_db_ErrEntry_createOneErr = q.F("Err")
const ___yo_db_ErrEntry_createOneErrDbRollback = q.F("ErrDbRollback")
const ___yo_db_ErrEntry_createOneHttpFullUri = q.F("HttpFullUri")
const ___yo_db_ErrEntry_createOneHttpUrlPath = q.F("HttpUrlPath")
const ___yo_db_ErrEntry_createOneId = q.F("Id")
const ___yo_db_ErrEntry_createOneJobRunId = q.F("JobRunId")
const ___yo_db_ErrEntry_createOneJobTaskId = q.F("JobTaskId")
const ___yo_db_ErrEntry_createOneNumCaught = q.F("NumCaught")
const ___yo_db_ErrEntry_deleteManyMax = q.F("Max")
const ___yo_db_ErrEntry_deleteManyOrderBy = q.F("OrderBy")
const ___yo_db_ErrEntry_deleteManyQuery = q.F("Query")
const ___yo_db_ErrEntry_deleteManyQueryFrom = q.F("QueryFrom")
const ___yo_db_ErrEntry_deleteOneId = q.F("Id")
const ___yo_db_ErrEntry_findByIdId = q.F("Id")
const ___yo_db_ErrEntry_findManyMax = q.F("Max")
const ___yo_db_ErrEntry_findManyOrderBy = q.F("OrderBy")
const ___yo_db_ErrEntry_findManyQuery = q.F("Query")
const ___yo_db_ErrEntry_findManyQueryFrom = q.F("QueryFrom")
const ___yo_db_ErrEntry_findOneMax = q.F("Max")
const ___yo_db_ErrEntry_findOneOrderBy = q.F("OrderBy")
const ___yo_db_ErrEntry_findOneQuery = q.F("Query")
const ___yo_db_ErrEntry_findOneQueryFrom = q.F("QueryFrom")
const ___yo_db_ErrEntry_updateManyChanges = q.F("Changes")
const ___yo_db_ErrEntry_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___yo_db_ErrEntry_updateManyMax = q.F("Max")
const ___yo_db_ErrEntry_updateManyOrderBy = q.F("OrderBy")
const ___yo_db_ErrEntry_updateManyQuery = q.F("Query")
const ___yo_db_ErrEntry_updateManyQueryFrom = q.F("QueryFrom")
const ___yo_db_ErrEntry_updateOneChangedFields = q.F("ChangedFields")
const ___yo_db_ErrEntry_updateOneChanges = q.F("Changes")
const ___yo_db_ErrEntry_updateOneId = q.F("Id")
const ___yo_db_JobDef_countMax = q.F("Max")
const ___yo_db_JobDef_countOrderBy = q.F("OrderBy")
const ___yo_db_JobDef_countQuery = q.F("Query")
const ___yo_db_JobDef_countQueryFrom = q.F("QueryFrom")
const ___yo_db_JobDef_createManyItems = q.F("Items")
const ___yo_db_JobDef_createOneAllowManualJobRuns = q.F("AllowManualJobRuns")
const ___yo_db_JobDef_createOneDeleteAfterDays = q.F("DeleteAfterDays")
const ___yo_db_JobDef_createOneDisabled = q.F("Disabled")
const ___yo_db_JobDef_createOneDtMade = q.F("DtMade")
const ___yo_db_JobDef_createOneDtMod = q.F("DtMod")
const ___yo_db_JobDef_createOneId = q.F("Id")
const ___yo_db_JobDef_createOneJobTypeId = q.F("JobTypeId")
const ___yo_db_JobDef_createOneMaxTaskRetries = q.F("MaxTaskRetries")
const ___yo_db_JobDef_createOneName = q.F("Name")
const ___yo_db_JobDef_createOneRunTasklessJobs = q.F("RunTasklessJobs")
const ___yo_db_JobDef_createOneSchedules = q.F("Schedules")
const ___yo_db_JobDef_createOneTimeoutSecsJobRunPrepAndFinalize = q.F("TimeoutSecsJobRunPrepAndFinalize")
const ___yo_db_JobDef_createOneTimeoutSecsTaskRun = q.F("TimeoutSecsTaskRun")
const ___yo_db_JobDef_deleteManyMax = q.F("Max")
const ___yo_db_JobDef_deleteManyOrderBy = q.F("OrderBy")
const ___yo_db_JobDef_deleteManyQuery = q.F("Query")
const ___yo_db_JobDef_deleteManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobDef_deleteOneId = q.F("Id")
const ___yo_db_JobDef_findByIdId = q.F("Id")
const ___yo_db_JobDef_findManyMax = q.F("Max")
const ___yo_db_JobDef_findManyOrderBy = q.F("OrderBy")
const ___yo_db_JobDef_findManyQuery = q.F("Query")
const ___yo_db_JobDef_findManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobDef_findOneMax = q.F("Max")
const ___yo_db_JobDef_findOneOrderBy = q.F("OrderBy")
const ___yo_db_JobDef_findOneQuery = q.F("Query")
const ___yo_db_JobDef_findOneQueryFrom = q.F("QueryFrom")
const ___yo_db_JobDef_updateManyChanges = q.F("Changes")
const ___yo_db_JobDef_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___yo_db_JobDef_updateManyMax = q.F("Max")
const ___yo_db_JobDef_updateManyOrderBy = q.F("OrderBy")
const ___yo_db_JobDef_updateManyQuery = q.F("Query")
const ___yo_db_JobDef_updateManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobDef_updateOneChangedFields = q.F("ChangedFields")
const ___yo_db_JobDef_updateOneChanges = q.F("Changes")
const ___yo_db_JobDef_updateOneId = q.F("Id")
const ___yo_db_JobRun_countMax = q.F("Max")
const ___yo_db_JobRun_countOrderBy = q.F("OrderBy")
const ___yo_db_JobRun_countQuery = q.F("Query")
const ___yo_db_JobRun_countQueryFrom = q.F("QueryFrom")
const ___yo_db_JobRun_createManyItems = q.F("Items")
const ___yo_db_JobRun_createOneAutoScheduled = q.F("AutoScheduled")
const ___yo_db_JobRun_createOneDetails = q.F("Details")
const ___yo_db_JobRun_createOneDtMade = q.F("DtMade")
const ___yo_db_JobRun_createOneDtMod = q.F("DtMod")
const ___yo_db_JobRun_createOneDueTime = q.F("DueTime")
const ___yo_db_JobRun_createOneDurationFinalizeSecs = q.F("DurationFinalizeSecs")
const ___yo_db_JobRun_createOneDurationPrepSecs = q.F("DurationPrepSecs")
const ___yo_db_JobRun_createOneFinishTime = q.F("FinishTime")
const ___yo_db_JobRun_createOneId = q.F("Id")
const ___yo_db_JobRun_createOneJobDef = q.F("JobDef")
const ___yo_db_JobRun_createOneJobTypeId = q.F("JobTypeId")
const ___yo_db_JobRun_createOneResults = q.F("Results")
const ___yo_db_JobRun_createOneScheduledNextAfter = q.F("ScheduledNextAfter")
const ___yo_db_JobRun_createOneStartTime = q.F("StartTime")
const ___yo_db_JobRun_createOneVersion = q.F("Version")
const ___yo_db_JobRun_deleteManyMax = q.F("Max")
const ___yo_db_JobRun_deleteManyOrderBy = q.F("OrderBy")
const ___yo_db_JobRun_deleteManyQuery = q.F("Query")
const ___yo_db_JobRun_deleteManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobRun_deleteOneId = q.F("Id")
const ___yo_db_JobRun_findByIdId = q.F("Id")
const ___yo_db_JobRun_findManyMax = q.F("Max")
const ___yo_db_JobRun_findManyOrderBy = q.F("OrderBy")
const ___yo_db_JobRun_findManyQuery = q.F("Query")
const ___yo_db_JobRun_findManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobRun_findOneMax = q.F("Max")
const ___yo_db_JobRun_findOneOrderBy = q.F("OrderBy")
const ___yo_db_JobRun_findOneQuery = q.F("Query")
const ___yo_db_JobRun_findOneQueryFrom = q.F("QueryFrom")
const ___yo_db_JobRun_updateManyChanges = q.F("Changes")
const ___yo_db_JobRun_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___yo_db_JobRun_updateManyMax = q.F("Max")
const ___yo_db_JobRun_updateManyOrderBy = q.F("OrderBy")
const ___yo_db_JobRun_updateManyQuery = q.F("Query")
const ___yo_db_JobRun_updateManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobRun_updateOneChangedFields = q.F("ChangedFields")
const ___yo_db_JobRun_updateOneChanges = q.F("Changes")
const ___yo_db_JobRun_updateOneId = q.F("Id")
const ___yo_db_JobTask_countMax = q.F("Max")
const ___yo_db_JobTask_countOrderBy = q.F("OrderBy")
const ___yo_db_JobTask_countQuery = q.F("Query")
const ___yo_db_JobTask_countQueryFrom = q.F("QueryFrom")
const ___yo_db_JobTask_createManyItems = q.F("Items")
const ___yo_db_JobTask_createOneAttempts = q.F("Attempts")
const ___yo_db_JobTask_createOneDetails = q.F("Details")
const ___yo_db_JobTask_createOneDtMade = q.F("DtMade")
const ___yo_db_JobTask_createOneDtMod = q.F("DtMod")
const ___yo_db_JobTask_createOneFinishTime = q.F("FinishTime")
const ___yo_db_JobTask_createOneId = q.F("Id")
const ___yo_db_JobTask_createOneJobRun = q.F("JobRun")
const ___yo_db_JobTask_createOneJobTypeId = q.F("JobTypeId")
const ___yo_db_JobTask_createOneResults = q.F("Results")
const ___yo_db_JobTask_createOneStartTime = q.F("StartTime")
const ___yo_db_JobTask_createOneVersion = q.F("Version")
const ___yo_db_JobTask_deleteManyMax = q.F("Max")
const ___yo_db_JobTask_deleteManyOrderBy = q.F("OrderBy")
const ___yo_db_JobTask_deleteManyQuery = q.F("Query")
const ___yo_db_JobTask_deleteManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobTask_deleteOneId = q.F("Id")
const ___yo_db_JobTask_findByIdId = q.F("Id")
const ___yo_db_JobTask_findManyMax = q.F("Max")
const ___yo_db_JobTask_findManyOrderBy = q.F("OrderBy")
const ___yo_db_JobTask_findManyQuery = q.F("Query")
const ___yo_db_JobTask_findManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobTask_findOneMax = q.F("Max")
const ___yo_db_JobTask_findOneOrderBy = q.F("OrderBy")
const ___yo_db_JobTask_findOneQuery = q.F("Query")
const ___yo_db_JobTask_findOneQueryFrom = q.F("QueryFrom")
const ___yo_db_JobTask_updateManyChanges = q.F("Changes")
const ___yo_db_JobTask_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___yo_db_JobTask_updateManyMax = q.F("Max")
const ___yo_db_JobTask_updateManyOrderBy = q.F("OrderBy")
const ___yo_db_JobTask_updateManyQuery = q.F("Query")
const ___yo_db_JobTask_updateManyQueryFrom = q.F("QueryFrom")
const ___yo_db_JobTask_updateOneChangedFields = q.F("ChangedFields")
const ___yo_db_JobTask_updateOneChanges = q.F("Changes")
const ___yo_db_JobTask_updateOneId = q.F("Id")
const ___yo_db_MailReq_countMax = q.F("Max")
const ___yo_db_MailReq_countOrderBy = q.F("OrderBy")
const ___yo_db_MailReq_countQuery = q.F("Query")
const ___yo_db_MailReq_countQueryFrom = q.F("QueryFrom")
const ___yo_db_MailReq_createManyItems = q.F("Items")
const ___yo_db_MailReq_createOneDtMade = q.F("DtMade")
const ___yo_db_MailReq_createOneDtMod = q.F("DtMod")
const ___yo_db_MailReq_createOneId = q.F("Id")
const ___yo_db_MailReq_createOneMailTo = q.F("MailTo")
const ___yo_db_MailReq_createOneTmplArgs = q.F("TmplArgs")
const ___yo_db_MailReq_createOneTmplId = q.F("TmplId")
const ___yo_db_MailReq_deleteManyMax = q.F("Max")
const ___yo_db_MailReq_deleteManyOrderBy = q.F("OrderBy")
const ___yo_db_MailReq_deleteManyQuery = q.F("Query")
const ___yo_db_MailReq_deleteManyQueryFrom = q.F("QueryFrom")
const ___yo_db_MailReq_deleteOneId = q.F("Id")
const ___yo_db_MailReq_findByIdId = q.F("Id")
const ___yo_db_MailReq_findManyMax = q.F("Max")
const ___yo_db_MailReq_findManyOrderBy = q.F("OrderBy")
const ___yo_db_MailReq_findManyQuery = q.F("Query")
const ___yo_db_MailReq_findManyQueryFrom = q.F("QueryFrom")
const ___yo_db_MailReq_findOneMax = q.F("Max")
const ___yo_db_MailReq_findOneOrderBy = q.F("OrderBy")
const ___yo_db_MailReq_findOneQuery = q.F("Query")
const ___yo_db_MailReq_findOneQueryFrom = q.F("QueryFrom")
const ___yo_db_MailReq_updateManyChanges = q.F("Changes")
const ___yo_db_MailReq_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___yo_db_MailReq_updateManyMax = q.F("Max")
const ___yo_db_MailReq_updateManyOrderBy = q.F("OrderBy")
const ___yo_db_MailReq_updateManyQuery = q.F("Query")
const ___yo_db_MailReq_updateManyQueryFrom = q.F("QueryFrom")
const ___yo_db_MailReq_updateOneChangedFields = q.F("ChangedFields")
const ___yo_db_MailReq_updateOneChanges = q.F("Changes")
const ___yo_db_MailReq_updateOneId = q.F("Id")
const ___yo_db_UserAuth_countMax = q.F("Max")
const ___yo_db_UserAuth_countOrderBy = q.F("OrderBy")
const ___yo_db_UserAuth_countQuery = q.F("Query")
const ___yo_db_UserAuth_countQueryFrom = q.F("QueryFrom")
const ___yo_db_UserAuth_createManyItems = q.F("Items")
const ___yo_db_UserAuth_createOneDtMade = q.F("DtMade")
const ___yo_db_UserAuth_createOneDtMod = q.F("DtMod")
const ___yo_db_UserAuth_createOneEmailAddr = q.F("EmailAddr")
const ___yo_db_UserAuth_createOneId = q.F("Id")
const ___yo_db_UserAuth_deleteManyMax = q.F("Max")
const ___yo_db_UserAuth_deleteManyOrderBy = q.F("OrderBy")
const ___yo_db_UserAuth_deleteManyQuery = q.F("Query")
const ___yo_db_UserAuth_deleteManyQueryFrom = q.F("QueryFrom")
const ___yo_db_UserAuth_deleteOneId = q.F("Id")
const ___yo_db_UserAuth_findByIdId = q.F("Id")
const ___yo_db_UserAuth_findManyMax = q.F("Max")
const ___yo_db_UserAuth_findManyOrderBy = q.F("OrderBy")
const ___yo_db_UserAuth_findManyQuery = q.F("Query")
const ___yo_db_UserAuth_findManyQueryFrom = q.F("QueryFrom")
const ___yo_db_UserAuth_findOneMax = q.F("Max")
const ___yo_db_UserAuth_findOneOrderBy = q.F("OrderBy")
const ___yo_db_UserAuth_findOneQuery = q.F("Query")
const ___yo_db_UserAuth_findOneQueryFrom = q.F("QueryFrom")
const ___yo_db_UserAuth_updateManyChanges = q.F("Changes")
const ___yo_db_UserAuth_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___yo_db_UserAuth_updateManyMax = q.F("Max")
const ___yo_db_UserAuth_updateManyOrderBy = q.F("OrderBy")
const ___yo_db_UserAuth_updateManyQuery = q.F("Query")
const ___yo_db_UserAuth_updateManyQueryFrom = q.F("QueryFrom")
const ___yo_db_UserAuth_updateOneChangedFields = q.F("ChangedFields")
const ___yo_db_UserAuth_updateOneChanges = q.F("Changes")
const ___yo_db_UserAuth_updateOneId = q.F("Id")
const ___yo_db_UserPwdReq_countMax = q.F("Max")
const ___yo_db_UserPwdReq_countOrderBy = q.F("OrderBy")
const ___yo_db_UserPwdReq_countQuery = q.F("Query")
const ___yo_db_UserPwdReq_countQueryFrom = q.F("QueryFrom")
const ___yo_db_UserPwdReq_createManyItems = q.F("Items")
const ___yo_db_UserPwdReq_createOneDoneMailReqId = q.F("DoneMailReqId")
const ___yo_db_UserPwdReq_createOneDtFinalized = q.F("DtFinalized")
const ___yo_db_UserPwdReq_createOneDtMade = q.F("DtMade")
const ___yo_db_UserPwdReq_createOneDtMod = q.F("DtMod")
const ___yo_db_UserPwdReq_createOneEmailAddr = q.F("EmailAddr")
const ___yo_db_UserPwdReq_createOneId = q.F("Id")
const ___yo_db_UserPwdReq_deleteManyMax = q.F("Max")
const ___yo_db_UserPwdReq_deleteManyOrderBy = q.F("OrderBy")
const ___yo_db_UserPwdReq_deleteManyQuery = q.F("Query")
const ___yo_db_UserPwdReq_deleteManyQueryFrom = q.F("QueryFrom")
const ___yo_db_UserPwdReq_deleteOneId = q.F("Id")
const ___yo_db_UserPwdReq_findByIdId = q.F("Id")
const ___yo_db_UserPwdReq_findManyMax = q.F("Max")
const ___yo_db_UserPwdReq_findManyOrderBy = q.F("OrderBy")
const ___yo_db_UserPwdReq_findManyQuery = q.F("Query")
const ___yo_db_UserPwdReq_findManyQueryFrom = q.F("QueryFrom")
const ___yo_db_UserPwdReq_findOneMax = q.F("Max")
const ___yo_db_UserPwdReq_findOneOrderBy = q.F("OrderBy")
const ___yo_db_UserPwdReq_findOneQuery = q.F("Query")
const ___yo_db_UserPwdReq_findOneQueryFrom = q.F("QueryFrom")
const ___yo_db_UserPwdReq_updateManyChanges = q.F("Changes")
const ___yo_db_UserPwdReq_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___yo_db_UserPwdReq_updateManyMax = q.F("Max")
const ___yo_db_UserPwdReq_updateManyOrderBy = q.F("OrderBy")
const ___yo_db_UserPwdReq_updateManyQuery = q.F("Query")
const ___yo_db_UserPwdReq_updateManyQueryFrom = q.F("QueryFrom")
const ___yo_db_UserPwdReq_updateOneChangedFields = q.F("ChangedFields")
const ___yo_db_UserPwdReq_updateOneChanges = q.F("Changes")
const ___yo_db_UserPwdReq_updateOneId = q.F("Id")
const ___yo_db_getTableName = q.F("Name")
