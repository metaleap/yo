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

}

/*

var httpEnsureAuthorized = HandlerFunc(func(ctx *Ctx) {
	req_path := ctx.ReqPath()
	if strBegins(req_path, "_/api/auth/") {
		return
	}
	_, err := ctx.httpAuthCheckAndSetCtxVals()
	if strBegins(req_path, "_/api/") && err != nil {
		ctx.httpErr(401, err)
	}
})

func (me *Ctx) httpAuthCheckAndSetCtxVals() (*AuthJwtPayload, error) {
	me.gCtx.Set("user_logged_in", false)

	jwtRaw, err := me.gCtx.Cookie(authJwtCookie)
	if err != nil { // http.ErrNoCookie
		return nil, err
	}
	token, err := jwt.ParseWithClaims(jwtRaw, &AuthJwtPayload{}, func(token *jwt.Token) (any, error) {
		return []byte(authJwtKey), nil
	})
	if err != nil {
		return nil, err
	}
	payload := token.Claims.(*AuthJwtPayload)
	me.gCtx.Set("user_logged_in", true)
	me.gCtx.Set("user_email", payload.StandardClaims.Subject)
	me.httpAuthSetCookie(jwtRaw)
	return payload, nil
}
*/
