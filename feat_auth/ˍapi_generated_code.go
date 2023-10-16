// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT
package yoauth

import reflect "reflect"
import util "yo/util"
import q "yo/db/query"

type _ = q.F // just in case of no other generated import users
type apiPkgInfo util.Void

func (apiPkgInfo) PkgName() string    { return "yoauth" }
func (me apiPkgInfo) PkgPath() string { return reflect.TypeOf(me).PkgPath() }

var PkgInfo = apiPkgInfo{}

const ErrAuthChangePassword_DbUpdateAcceptedWithoutErrButNotStoredEither util.Err = "AuthChangePassword_DbUpdateAcceptedWithoutErrButNotStoredEither"
const ErrAuthChangePassword_Forbidden util.Err = "AuthChangePassword_Forbidden"
const ErrAuthChangePassword_NewPasswordExpectedToDiffer util.Err = "AuthChangePassword_NewPasswordExpectedToDiffer"
const ErrAuthChangePassword_NewPasswordInvalid util.Err = "AuthChangePassword_NewPasswordInvalid"
const ErrAuthChangePassword_NewPasswordTooLong util.Err = "AuthChangePassword_NewPasswordTooLong"
const ErrAuthChangePassword_NewPasswordTooShort util.Err = "AuthChangePassword_NewPasswordTooShort"
const ErrAuthLogin_AccountDoesNotExist util.Err = "AuthLogin_AccountDoesNotExist"
const ErrAuthLogin_EmailInvalid util.Err = "AuthLogin_EmailInvalid"
const ErrAuthLogin_EmailRequiredButMissing util.Err = "AuthLogin_EmailRequiredButMissing"
const ErrAuthLogin_OkButFailedToCreateSignedToken util.Err = "AuthLogin_OkButFailedToCreateSignedToken"
const ErrAuthLogin_WrongPassword util.Err = "AuthLogin_WrongPassword"
const ErrDbUpdate_ExpectedChangesForUpdate util.Err = "DbUpdate_ExpectedChangesForUpdate"
const ErrDbUpdate_ExpectedQueryForUpdate util.Err = "DbUpdate_ExpectedQueryForUpdate"
const ErrAuthRegister_DbInsertAcceptedWithoutErrButNotStoredEither util.Err = "AuthRegister_DbInsertAcceptedWithoutErrButNotStoredEither"
const ErrAuthRegister_EmailAddrAlreadyExists util.Err = "AuthRegister_EmailAddrAlreadyExists"
const ErrAuthRegister_EmailInvalid util.Err = "AuthRegister_EmailInvalid"
const ErrAuthRegister_EmailRequiredButMissing util.Err = "AuthRegister_EmailRequiredButMissing"
const ErrAuthRegister_PasswordInvalid util.Err = "AuthRegister_PasswordInvalid"
const ErrAuthRegister_PasswordTooLong util.Err = "AuthRegister_PasswordTooLong"
const ErrAuthRegister_PasswordTooShort util.Err = "AuthRegister_PasswordTooShort"
const AuthChangePasswordEmailAddr = q.F("EmailAddr")
const AuthChangePasswordPasswordNewPlain = q.F("PasswordNewPlain")
const AuthChangePasswordPasswordPlain = q.F("PasswordPlain")
const AuthLoginEmailAddr = q.F("EmailAddr")
const AuthLoginPasswordPlain = q.F("PasswordPlain")
const AuthRegisterEmailAddr = q.F("EmailAddr")
const AuthRegisterPasswordPlain = q.F("PasswordPlain")
