package yofeat_auth

import (
	. "yo/ctx"
	yodb "yo/db"
	yoserve "yo/server"
	. "yo/util"
	"yo/util/str"

	"golang.org/x/crypto/bcrypt"
)

type UserAccount struct {
	Id      yodb.I64
	Created yodb.DateTime

	EmailAddr      yodb.Text
	passwordHashed yodb.Bytes
}

func init() {
	yodb.Ensure[UserAccount, UserAccountField](false, "", nil)
	yoserve.API["authRegister"] = yoserve.Method(apiUserRegister)
}

func UserRegister(ctx *Ctx, emailAddr string, passwordPlain string) yodb.I64 {
	emailAddr, passwordPlain = str.Trim(emailAddr), str.Trim(passwordPlain)
	if emailAddr == "" {
		panic(Err("UserRegisterEmailRequiredButMissing"))
	}
	if passwordPlain == "" {
		panic(Err("UserRegisterPasswordRequiredButMissing"))
	}
	ctx.DbTx(yodb.DB)
	if yodb.Exists[UserAccount](ctx, UserAccountColEmailAddr.Equal(emailAddr)) {
		panic(Err("UserRegisterEmailAddrAlreadyExists"))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return yodb.CreateOne[UserAccount](ctx, &UserAccount{
		EmailAddr:      yodb.Text(emailAddr),
		passwordHashed: hash,
	})
}
