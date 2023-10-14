// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT
package yoauth

import (
	util "yo/util"
)

type apiPkgInfo util.Void

func (apiPkgInfo) PkgName() string { return "yoauth" }

var PkgInfo = apiPkgInfo{}

const ErrAuthChangePassword_ChangesNotStored util.Err = "AuthChangePassword_ChangesNotStored"
const ErrAuthChangePassword_Forbidden util.Err = "AuthChangePassword_Forbidden"
const ErrAuthChangePassword_NewPasswordInvalid util.Err = "AuthChangePassword_NewPasswordInvalid"
const ErrAuthChangePassword_NewPasswordRequiredButMissing util.Err = "AuthChangePassword_NewPasswordRequiredButMissing"
const ErrAuthChangePassword_NewPasswordSameAsOld util.Err = "AuthChangePassword_NewPasswordSameAsOld"
const ErrAuthChangePassword_NewPasswordTooLong util.Err = "AuthChangePassword_NewPasswordTooLong"
const ErrAuthChangePassword_NewPasswordTooShort util.Err = "AuthChangePassword_NewPasswordTooShort"
const ErrAuthLogin_AccountDoesNotExist util.Err = "AuthLogin_AccountDoesNotExist"
const ErrAuthLogin_EmailInvalid util.Err = "AuthLogin_EmailInvalid"
const ErrAuthLogin_EmailRequiredButMissing util.Err = "AuthLogin_EmailRequiredButMissing"
const ErrAuthLogin_OkButFailedToCreateSignedToken util.Err = "AuthLogin_OkButFailedToCreateSignedToken"
const ErrAuthLogin_PasswordRequiredButMissing util.Err = "AuthLogin_PasswordRequiredButMissing"
const ErrAuthLogin_WrongPassword util.Err = "AuthLogin_WrongPassword"
const ErrAuthRegister_EmailAddrAlreadyExists util.Err = "AuthRegister_EmailAddrAlreadyExists"
const ErrAuthRegister_EmailInvalid util.Err = "AuthRegister_EmailInvalid"
const ErrAuthRegister_EmailRequiredButMissing util.Err = "AuthRegister_EmailRequiredButMissing"
const ErrAuthRegister_PasswordInvalid util.Err = "AuthRegister_PasswordInvalid"
const ErrAuthRegister_PasswordRequiredButMissing util.Err = "AuthRegister_PasswordRequiredButMissing"
const ErrAuthRegister_PasswordTooLong util.Err = "AuthRegister_PasswordTooLong"
const ErrAuthRegister_PasswordTooShort util.Err = "AuthRegister_PasswordTooShort"
const ErrAuthRegister_WhileLoggedIn util.Err = "AuthRegister_WhileLoggedIn"
