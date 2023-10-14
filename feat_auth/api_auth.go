package yoauth

import (
	. "yo/cfg"
	. "yo/ctx"
	. "yo/srv"
	. "yo/util"
)

const (
	CtxKey            = "yoUser"
	HttpUserHeader    = "X-Yo-User"
	HttpJwtCookieName = "t"

	MethodPathLogin          = "authLogin"
	MethodPathLogout         = "authLogout"
	MethodPathRegister       = "authRegister"
	MethodPathChangePassword = "authChangePassword"
)

func init() {
	Apis(ApiMethods{
		MethodPathLogin:          Api(apiUserLogin, PkgInfo, "OkButFailedToCreateSignedToken", "EmailRequiredButMissing", "EmailInvalid", "PasswordRequiredButMissing", "AccountDoesNotExist", "WrongPassword"),
		MethodPathLogout:         Api(apiUserLogout, PkgInfo),
		MethodPathRegister:       Api(apiUserRegister, PkgInfo, "WhileLoggedIn", "EmailRequiredButMissing", "EmailInvalid", "EmailAddrAlreadyExists", "PasswordRequiredButMissing", "PasswordTooShort", "PasswordTooLong", "PasswordInvalid"),
		MethodPathChangePassword: Api(apiChangePassword, PkgInfo, ":"+MethodPathLogin, "Forbidden", "NewPasswordRequiredButMissing", "NewPasswordTooShort", "NewPasswordSameAsOld", "NewPasswordTooLong", "NewPasswordInvalid", "ChangesNotStored"),
	})
	PreServes = append(PreServes, PreServe{Name: "authCheck", Do: func(ctx *Ctx) {
		httpSetUser(ctx, ctx.HttpGetCookie(HttpJwtCookieName))
	}})
}

type ApiAccountPayload struct {
	EmailAddr     string
	PasswordPlain string
}

type ApiTokenPayload struct {
	JwtSignedToken string
}

func apiUserRegister(this *ApiCtx[ApiAccountPayload, struct {
	Id int64
}]) {
	if this.Ctx.GetStr(CtxKey) != "" {
		panic(ErrAuthRegisterWhileLoggedIn)
	}
	httpSetUser(this.Ctx, "")
	this.Ret.Id = int64(UserRegister(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain))
}

func apiUserLogin(this *ApiCtx[ApiAccountPayload, Void]) {
	httpSetUser(this.Ctx, "")
	_, jwt_token := UserLogin(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain)
	jwt_signed, err := jwt_token.SignedString(jwtKey)
	if err != nil {
		panic(ErrAuthLoginOkButFailedToCreateSignedToken)
	}
	httpSetUser(this.Ctx, jwt_signed)
}

func apiUserLogout(ctx *ApiCtx[Void, Void]) {
	httpSetUser(ctx.Ctx, "")
}

func apiChangePassword(this *ApiCtx[struct {
	ApiAccountPayload
	PasswordNewPlain string
}, Void]) {
	if user_email_addr := this.Ctx.GetStr(CtxKey); user_email_addr != "" && user_email_addr != this.Args.EmailAddr {
		panic(ErrAuthChangePasswordForbidden)
	}
	httpSetUser(this.Ctx, "")
	if !UserChangePassword(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain, this.Args.PasswordNewPlain) {
		panic(ErrAuthChangePasswordChangesNotStored)
	}
}

func httpSetUser(ctx *Ctx, jwtRaw string) {
	user_email_addr := ""
	if jwtRaw != "" {
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
