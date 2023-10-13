package yofeat_auth

import (
	"net/url"

	. "yo/cfg"
	. "yo/ctx"
	yoserve "yo/server"
	. "yo/util"
)

func init() {
	yoserve.API["authRegister"] = yoserve.Method(apiUserRegister)
	yoserve.API["authLogin"] = yoserve.Method(apiUserLogin)
	yoserve.PreServe = append(yoserve.PreServe, httpCheckJwtCookie)
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

func apiUserLogin(ctx *Ctx, args *ApiAccountPayload, ret *Void) any {
	httpSetJwtCookie(ctx, UserLogin(ctx, args.EmailAddr, args.PasswordPlain))
	return ret
}

const jwtCookieName = "t"

func httpSetJwtCookie(ctx *Ctx, jwtRaw string) {
	ctx.HttpSetCookie(jwtCookieName, url.QueryEscape(jwtRaw), Cfg.YO_AUTH_JWT_EXPIRY_DAYS)
}

func httpCheckJwtCookie(ctx *Ctx) {
	ctx.Set("user_email_addr", "")
	jwt_raw := ctx.HttpGetCookie(jwtCookieName)
	if jwt_raw != "" {
		if jwt_payload := UserVerify(ctx, jwt_raw); jwt_payload != nil {
			ctx.Set("user_email_addr", jwt_payload.StandardClaims.Subject)
		} else {
			jwt_raw = ""
		}
	}
	httpSetJwtCookie(ctx, jwt_raw)
}
