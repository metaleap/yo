package yofeat_auth

import (
	. "yo/ctx"
	yoserve "yo/server"
)

func init() {
	yoserve.API["authRegister"] = yoserve.Method(apiUserRegister)
	yoserve.API["authLogin"] = yoserve.Method(apiUserLogin)
}

type ApiAccountPayload struct {
	EmailAddr     string
	PasswordPlain string
}

type ApiTokenPayload struct {
	JwtSignedToken string
}

func apiUserRegister(ctx *Ctx, args *ApiAccountPayload, ret *struct {
	Id int64
}) any {
	ret.Id = int64(UserRegister(ctx, args.EmailAddr, args.PasswordPlain))
	return ret
}

func apiUserLogin(ctx *Ctx, args *ApiAccountPayload, ret *ApiTokenPayload) any {
	ret.JwtSignedToken = UserLogin(ctx, args.EmailAddr, args.PasswordPlain)
	return ret
}
