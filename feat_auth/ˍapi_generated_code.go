// Code generated by `yo/srv/codegen_apistuff.go` DO NOT EDIT
package yoauth

import (
	util "yo/util"
)

type apiPkgInfo struct{}

func (apiPkgInfo) PkgName() string { return "yoauth" }

var PkgInfo = apiPkgInfo{}

const ErrAuthLoginBazFail util.Err = "AuthLoginBazFail"
const ErrAuthLoginFooFail util.Err = "AuthLoginFooFail"
const ErrAuthLoginBarFail util.Err = "AuthLoginBarFail"
