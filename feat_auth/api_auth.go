package yofeat_auth

import (
	. "yo/cfg"
	. "yo/ctx"
	yoserve "yo/server"
	. "yo/util"
)

const (
	CtxKey                   = "yoUser"
	HttpUserHeader           = "X-Yo-User"
	HttpJwtCookieName        = "t"
	MethodPathLogin          = "authLogin"
	MethodPathLogout         = "authLogout"
	MethodPathRegister       = "authRegister"
	MethodPathChangePassword = "authChangePassword"
)

func init() {
	yoserve.API[MethodPathLogout] = yoserve.Method(apiUserLogout)
	yoserve.API[MethodPathLogin] = yoserve.Method(apiUserLogin)
	yoserve.API[MethodPathRegister] = yoserve.Method(apiUserRegister)
	yoserve.API[MethodPathChangePassword] = yoserve.Method(apiChangePassword)
	yoserve.PreServe["authCheck"] = httpCheckAndSet
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
	if ctx.GetStr(CtxKey) != "" {
		panic(Err("UserRegisterWhileLoggedIn"))
	}
	httpSetUser(ctx, "")
	ret.Id = int64(UserRegister(ctx, args.EmailAddr, args.PasswordPlain))
	return ret
}

func apiUserLogin(ctx *Ctx, args *ApiAccountPayload, ret *Void) any {
	httpSetUser(ctx, "")
	_, jwt_token := UserLogin(ctx, args.EmailAddr, args.PasswordPlain)
	jwt_signed, err := jwt_token.SignedString(jwtKey)
	if err != nil {
		panic(Err("UserLoginOkButFailedToCreateSignedToken"))
	}
	httpSetUser(ctx, jwt_signed)
	return ret
}

func apiUserLogout(ctx *Ctx, args *Void, ret *Void) any {
	httpSetUser(ctx, "")
	return ret
}

func apiChangePassword(ctx *Ctx, args *struct {
	ApiAccountPayload
	PasswordNewPlain string
}, ret *struct {
	Did bool
}) any {
	httpSetUser(ctx, "")
	ret.Did = UserChangePassword(ctx, args.EmailAddr, args.PasswordPlain, args.PasswordNewPlain)
	return ret
}

func httpSetUser(ctx *Ctx, jwtRaw string) {
	user_email_addr := ""
	if (jwtRaw != "") && (ctx.Http.UrlPath != MethodPathLogout) {
		if jwt_payload := UserVerify(ctx, jwtRaw); jwt_payload == nil {
			jwtRaw = ""
		} else {
			user_email_addr = jwt_payload.StandardClaims.Subject
		}
	}
	ctx.Set(CtxKey, user_email_addr)
	ctx.Http.Resp.Header().Set(HttpUserHeader, user_email_addr)
	ctx.HttpSetCookie(HttpJwtCookieName, jwtRaw, Cfg.YO_AUTH_JWT_EXPIRY_DAYS)
}

func httpCheckAndSet(ctx *Ctx) {
	httpSetUser(ctx, ctx.HttpGetCookie(HttpJwtCookieName))
}
