package yoauth

import (
	. "yo/cfg"
	. "yo/ctx"
	q "yo/db/query"
	. "yo/srv"
	. "yo/util"
	"yo/util/str"

	yodb "yo/db"
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

var (
	isEmailishEnough = q.Via(str.IsEmailishEnough)
)

func init() {
	Apis(ApiMethods{
		MethodPathLogout: Api(apiUserLogout, PkgInfo),

		MethodPathLogin: Api(apiUserLogin, PkgInfo,
			Fails{Err: "EmailRequiredButMissing", If: AuthRegisterEmailAddr.Equal("")},
			Fails{Err: "EmailInvalid", If: isEmailishEnough(AuthLoginEmailAddr).Not()},
			Fails{Err: "WrongPassword",
				If: AuthLoginPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN).Or(
					AuthLoginPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN))},
		).CouldFailWith("OkButFailedToCreateSignedToken", "AccountDoesNotExist"),

		MethodPathRegister: Api(apiUserRegister, PkgInfo,
			Fails{Err: "EmailRequiredButMissing", If: AuthRegisterEmailAddr.Equal("")},
			Fails{Err: "EmailInvalid", If: isEmailishEnough(AuthRegisterEmailAddr).Not()},
			Fails{Err: "PasswordTooShort", If: AuthRegisterPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
			Fails{Err: "PasswordTooLong", If: AuthRegisterPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
		).CouldFailWith("EmailAddrAlreadyExists", "PasswordInvalid"),

		MethodPathChangePassword: Api(apiChangePassword, PkgInfo,
			Fails{Err: "NewPasswordSameAsOld", If: AuthChangePasswordPasswordNewPlain.Equal(AuthChangePasswordPasswordPlain)},
			Fails{Err: "NewPasswordTooShort", If: AuthChangePasswordPasswordNewPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
			Fails{Err: "NewPasswordTooLong", If: AuthChangePasswordPasswordNewPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
		).CouldFailWith(":"+yodb.ErrSetDbUpdate, ":"+MethodPathLogin, "Forbidden", "NewPasswordInvalid", "ChangesNotStored"),
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
	httpSetUser(this.Ctx, "")
	this.Ret.Id = int64(UserRegister(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain))
}

func apiUserLogin(this *ApiCtx[ApiAccountPayload, Void]) {
	httpSetUser(this.Ctx, "")
	_, jwt_token := UserLogin(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain)
	jwt_signed, err := jwt_token.SignedString(jwtKey)
	if err != nil {
		panic(ErrAuthLogin_OkButFailedToCreateSignedToken)
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
	if user_email_addr := this.Ctx.GetStr(CtxKey); user_email_addr != this.Args.EmailAddr {
		panic(ErrAuthChangePassword_Forbidden)
	}
	httpSetUser(this.Ctx, "")
	if !UserChangePassword(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain, this.Args.PasswordNewPlain) {
		panic(ErrAuthChangePassword_ChangesNotStored)
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
