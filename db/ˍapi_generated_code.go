// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT
package yodb

import reflect "reflect"
import util "yo/util"
import q "yo/db/query"

type _ = q.F // just in case of no other generated import users
type apiPkgInfo util.Void

func (apiPkgInfo) PkgName() string    { return "yodb" }
func (me apiPkgInfo) PkgPath() string { return reflect.TypeOf(me).PkgPath() }

var PkgInfo = apiPkgInfo{}

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
const Err___db_BoxSettings_updateOne_ExpectedIdGreater0 util.Err = "___db_BoxSettings_updateOne_ExpectedIdGreater0"
const Err___db_User_updateOne_ExpectedIdGreater0 util.Err = "___db_User_updateOne_ExpectedIdGreater0"
const Err___db_UserAuth_updateOne_ExpectedIdGreater0 util.Err = "___db_UserAuth_updateOne_ExpectedIdGreater0"
const ___db_BoxSettings_countMax = q.F("Max")
const ___db_BoxSettings_countOrderBy = q.F("OrderBy")
const ___db_BoxSettings_countQuery = q.F("Query")
const ___db_BoxSettings_countQueryFrom = q.F("QueryFrom")
const ___db_BoxSettings_createManyItems = q.F("Items")
const ___db_BoxSettings_createOneCreated = q.F("Created")
const ___db_BoxSettings_createOneEmailOnUploads = q.F("EmailOnUploads")
const ___db_BoxSettings_createOneHideBranding = q.F("HideBranding")
const ___db_BoxSettings_createOneId = q.F("Id")
const ___db_BoxSettings_createOneInstructions = q.F("Instructions")
const ___db_BoxSettings_createOneMaxFileSizeMb = q.F("MaxFileSizeMb")
const ___db_BoxSettings_createOneMaxFilesPerUpload = q.F("MaxFilesPerUpload")
const ___db_BoxSettings_createOneRetainDays = q.F("RetainDays")
const ___db_BoxSettings_createOneTestMap = q.F("TestMap")
const ___db_BoxSettings_createOneTitle = q.F("Title")
const ___db_BoxSettings_createOneZipFilesPerUpload = q.F("ZipFilesPerUpload")
const ___db_BoxSettings_deleteManyMax = q.F("Max")
const ___db_BoxSettings_deleteManyOrderBy = q.F("OrderBy")
const ___db_BoxSettings_deleteManyQuery = q.F("Query")
const ___db_BoxSettings_deleteManyQueryFrom = q.F("QueryFrom")
const ___db_BoxSettings_deleteOneId = q.F("Id")
const ___db_BoxSettings_findByIdId = q.F("Id")
const ___db_BoxSettings_findManyMax = q.F("Max")
const ___db_BoxSettings_findManyOrderBy = q.F("OrderBy")
const ___db_BoxSettings_findManyQuery = q.F("Query")
const ___db_BoxSettings_findManyQueryFrom = q.F("QueryFrom")
const ___db_BoxSettings_findOneMax = q.F("Max")
const ___db_BoxSettings_findOneOrderBy = q.F("OrderBy")
const ___db_BoxSettings_findOneQuery = q.F("Query")
const ___db_BoxSettings_findOneQueryFrom = q.F("QueryFrom")
const ___db_BoxSettings_updateManyChanges = q.F("Changes")
const ___db_BoxSettings_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___db_BoxSettings_updateManyMax = q.F("Max")
const ___db_BoxSettings_updateManyOrderBy = q.F("OrderBy")
const ___db_BoxSettings_updateManyQuery = q.F("Query")
const ___db_BoxSettings_updateManyQueryFrom = q.F("QueryFrom")
const ___db_BoxSettings_updateOneChanges = q.F("Changes")
const ___db_BoxSettings_updateOneId = q.F("Id")
const ___db_BoxSettings_updateOneIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___db_User_countMax = q.F("Max")
const ___db_User_countOrderBy = q.F("OrderBy")
const ___db_User_countQuery = q.F("Query")
const ___db_User_countQueryFrom = q.F("QueryFrom")
const ___db_User_createManyItems = q.F("Items")
const ___db_User_createOneCreated = q.F("Created")
const ___db_User_createOneEmailAddr = q.F("EmailAddr")
const ___db_User_createOneId = q.F("Id")
const ___db_User_createOneName = q.F("Name")
const ___db_User_createOneSignUpMailSent = q.F("SignUpMailSent")
const ___db_User_deleteManyMax = q.F("Max")
const ___db_User_deleteManyOrderBy = q.F("OrderBy")
const ___db_User_deleteManyQuery = q.F("Query")
const ___db_User_deleteManyQueryFrom = q.F("QueryFrom")
const ___db_User_deleteOneId = q.F("Id")
const ___db_User_findByIdId = q.F("Id")
const ___db_User_findManyMax = q.F("Max")
const ___db_User_findManyOrderBy = q.F("OrderBy")
const ___db_User_findManyQuery = q.F("Query")
const ___db_User_findManyQueryFrom = q.F("QueryFrom")
const ___db_User_findOneMax = q.F("Max")
const ___db_User_findOneOrderBy = q.F("OrderBy")
const ___db_User_findOneQuery = q.F("Query")
const ___db_User_findOneQueryFrom = q.F("QueryFrom")
const ___db_User_updateManyChanges = q.F("Changes")
const ___db_User_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___db_User_updateManyMax = q.F("Max")
const ___db_User_updateManyOrderBy = q.F("OrderBy")
const ___db_User_updateManyQuery = q.F("Query")
const ___db_User_updateManyQueryFrom = q.F("QueryFrom")
const ___db_User_updateOneChanges = q.F("Changes")
const ___db_User_updateOneId = q.F("Id")
const ___db_User_updateOneIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___db_UserAuth_countMax = q.F("Max")
const ___db_UserAuth_countOrderBy = q.F("OrderBy")
const ___db_UserAuth_countQuery = q.F("Query")
const ___db_UserAuth_countQueryFrom = q.F("QueryFrom")
const ___db_UserAuth_createManyItems = q.F("Items")
const ___db_UserAuth_createOneCreated = q.F("Created")
const ___db_UserAuth_createOneEmailAddr = q.F("EmailAddr")
const ___db_UserAuth_createOneId = q.F("Id")
const ___db_UserAuth_deleteManyMax = q.F("Max")
const ___db_UserAuth_deleteManyOrderBy = q.F("OrderBy")
const ___db_UserAuth_deleteManyQuery = q.F("Query")
const ___db_UserAuth_deleteManyQueryFrom = q.F("QueryFrom")
const ___db_UserAuth_deleteOneId = q.F("Id")
const ___db_UserAuth_findByIdId = q.F("Id")
const ___db_UserAuth_findManyMax = q.F("Max")
const ___db_UserAuth_findManyOrderBy = q.F("OrderBy")
const ___db_UserAuth_findManyQuery = q.F("Query")
const ___db_UserAuth_findManyQueryFrom = q.F("QueryFrom")
const ___db_UserAuth_findOneMax = q.F("Max")
const ___db_UserAuth_findOneOrderBy = q.F("OrderBy")
const ___db_UserAuth_findOneQuery = q.F("Query")
const ___db_UserAuth_findOneQueryFrom = q.F("QueryFrom")
const ___db_UserAuth_updateManyChanges = q.F("Changes")
const ___db_UserAuth_updateManyIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___db_UserAuth_updateManyMax = q.F("Max")
const ___db_UserAuth_updateManyOrderBy = q.F("OrderBy")
const ___db_UserAuth_updateManyQuery = q.F("Query")
const ___db_UserAuth_updateManyQueryFrom = q.F("QueryFrom")
const ___db_UserAuth_updateOneChanges = q.F("Changes")
const ___db_UserAuth_updateOneId = q.F("Id")
const ___db_UserAuth_updateOneIncludingEmptyOrMissingFields = q.F("IncludingEmptyOrMissingFields")
const ___db_listTablesName = q.F("Name")
