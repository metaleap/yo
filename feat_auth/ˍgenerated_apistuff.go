// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT
package yoauth

import reflect "reflect"
import yosrv "yo/srv"
import util "yo/util"
import q "yo/db/query"

type _ = q.F // just in case of no other generated import users
type apiPkgInfo util.Void

func (apiPkgInfo) PkgName() string    { return "yoauth" }
func (me apiPkgInfo) PkgPath() string { return reflect.TypeOf(me).PkgPath() }

var yoauthPkg = apiPkgInfo{}

func api[TIn any, TOut any](f func(*yosrv.ApiCtx[TIn, TOut]), failIfs ...yosrv.Fails) yosrv.ApiMethod {
	return yosrv.Api[TIn, TOut](f, failIfs...).From(yoauthPkg)
}

const ErrDbUpdate_ExpectedChangesForUpdate util.Err = "DbUpdate_ExpectedChangesForUpdate"
const ErrDbUpdate_ExpectedQueryForUpdate util.Err = "DbUpdate_ExpectedQueryForUpdate"
const Err___yo_authChangePassword_NewPasswordExpectedToDiffer util.Err = "___yo_authChangePassword_NewPasswordExpectedToDiffer"
const Err___yo_authChangePassword_NewPasswordInvalid util.Err = "___yo_authChangePassword_NewPasswordInvalid"
const Err___yo_authChangePassword_NewPasswordTooLong util.Err = "___yo_authChangePassword_NewPasswordTooLong"
const Err___yo_authChangePassword_NewPasswordTooShort util.Err = "___yo_authChangePassword_NewPasswordTooShort"
const Err___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist util.Err = "___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist"
const Err___yo_authLoginOrFinalizePwdReset_EmailInvalid util.Err = "___yo_authLoginOrFinalizePwdReset_EmailInvalid"
const Err___yo_authLoginOrFinalizePwdReset_EmailRequiredButMissing util.Err = "___yo_authLoginOrFinalizePwdReset_EmailRequiredButMissing"
const Err___yo_authLoginOrFinalizePwdReset_NewPasswordExpectedToDiffer util.Err = "___yo_authLoginOrFinalizePwdReset_NewPasswordExpectedToDiffer"
const Err___yo_authLoginOrFinalizePwdReset_NewPasswordInvalid util.Err = "___yo_authLoginOrFinalizePwdReset_NewPasswordInvalid"
const Err___yo_authLoginOrFinalizePwdReset_NewPasswordTooLong util.Err = "___yo_authLoginOrFinalizePwdReset_NewPasswordTooLong"
const Err___yo_authLoginOrFinalizePwdReset_NewPasswordTooShort util.Err = "___yo_authLoginOrFinalizePwdReset_NewPasswordTooShort"
const Err___yo_authLoginOrFinalizePwdReset_OkButFailedToCreateSignedToken util.Err = "___yo_authLoginOrFinalizePwdReset_OkButFailedToCreateSignedToken"
const Err___yo_authLoginOrFinalizePwdReset_WrongPassword util.Err = "___yo_authLoginOrFinalizePwdReset_WrongPassword"
const Err___yo_authRegister_EmailAddrAlreadyExists util.Err = "___yo_authRegister_EmailAddrAlreadyExists"
const Err___yo_authRegister_EmailInvalid util.Err = "___yo_authRegister_EmailInvalid"
const Err___yo_authRegister_EmailRequiredButMissing util.Err = "___yo_authRegister_EmailRequiredButMissing"
const Err___yo_authRegister_PasswordInvalid util.Err = "___yo_authRegister_PasswordInvalid"
const Err___yo_authRegister_PasswordTooLong util.Err = "___yo_authRegister_PasswordTooLong"
const Err___yo_authRegister_PasswordTooShort util.Err = "___yo_authRegister_PasswordTooShort"
const ___yo_authChangePasswordEmailAddr = q.F("EmailAddr")
const ___yo_authChangePasswordPassword2Plain = q.F("Password2Plain")
const ___yo_authChangePasswordPasswordPlain = q.F("PasswordPlain")
const ___yo_authLoginOrFinalizePwdResetEmailAddr = q.F("EmailAddr")
const ___yo_authLoginOrFinalizePwdResetPassword2Plain = q.F("Password2Plain")
const ___yo_authLoginOrFinalizePwdResetPasswordPlain = q.F("PasswordPlain")
const ___yo_authRegisterEmailAddr = q.F("EmailAddr")
const ___yo_authRegisterPassword2Plain = q.F("Password2Plain")
const ___yo_authRegisterPasswordPlain = q.F("PasswordPlain")
