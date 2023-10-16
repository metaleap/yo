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

	MethodPathLogin          = "__/admin/authLogin"
	MethodPathLogout         = "__/admin/authLogout"
	MethodPathRegister       = "__/admin/authRegister"
	MethodPathChangePassword = "__/admin/authChangePassword"
)

var (
	isEmailishEnough = q.Via(str.IsEmailishEnough)
)

func init() {
	Apis(ApiMethods{
		MethodPathLogout: Api(ApiUserLogout, PkgInfo),

		MethodPathLogin: Api(ApiUserLogin, PkgInfo,
			Fails{Err: "EmailRequiredButMissing", If: ___admin_authRegisterEmailAddr.Equal("")},
			Fails{Err: "EmailInvalid", If: isEmailishEnough(___admin_authLoginEmailAddr).Not()},
			Fails{Err: "WrongPassword",
				If: ___admin_authLoginPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN).Or(
					___admin_authLoginPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN))},
		).CouldFailWith("OkButFailedToCreateSignedToken", "AccountDoesNotExist"),

		MethodPathRegister: Api(ApiUserRegister, PkgInfo,
			Fails{Err: "EmailRequiredButMissing", If: ___admin_authRegisterEmailAddr.Equal("")},
			Fails{Err: "EmailInvalid", If: isEmailishEnough(___admin_authRegisterEmailAddr).Not()},
			Fails{Err: "PasswordTooShort", If: ___admin_authRegisterPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
			Fails{Err: "PasswordTooLong", If: ___admin_authRegisterPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
		).CouldFailWith("EmailAddrAlreadyExists", "PasswordInvalid", "DbInsertAcceptedWithoutErrButNotStoredEither"),

		MethodPathChangePassword: Api(apiChangePassword, PkgInfo,
			Fails{Err: "NewPasswordExpectedToDiffer", If: ___admin_authChangePasswordPasswordNewPlain.Equal(___admin_authChangePasswordPasswordPlain)},
			Fails{Err: "NewPasswordTooShort", If: ___admin_authChangePasswordPasswordNewPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
			Fails{Err: "NewPasswordTooLong", If: ___admin_authChangePasswordPasswordNewPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
		).CouldFailWith(":"+yodb.ErrSetDbUpdate, ":"+MethodPathLogin, "Forbidden", "NewPasswordInvalid", "DbUpdateAcceptedWithoutErrButNotStoredEither"),
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

func ApiUserRegister(this *ApiCtx[ApiAccountPayload, struct {
	Id yodb.I64
}]) {
	httpSetUser(this.Ctx, "")
	this.Ret.Id = UserRegister(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain)
}

func ApiUserLogin(this *ApiCtx[ApiAccountPayload, Void]) {
	httpSetUser(this.Ctx, "")
	_, jwt_token := UserLogin(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain)
	jwt_signed, err := jwt_token.SignedString(jwtKey)
	if err != nil {
		// panic(ErrAuthLogin_OkButFailedToCreateSignedToken)
	}
	httpSetUser(this.Ctx, jwt_signed)
}

func ApiUserLogout(ctx *ApiCtx[Void, Void]) {
	httpSetUser(ctx.Ctx, "")
}

func apiChangePassword(this *ApiCtx[struct {
	ApiAccountPayload
	PasswordNewPlain string
}, Void]) {
	if user_email_addr := this.Ctx.GetStr(CtxKey); user_email_addr != this.Args.EmailAddr {
		// panic(ErrAuthChangePassword_Forbidden)
	}
	httpSetUser(this.Ctx, "")
	UserChangePassword(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain, this.Args.PasswordNewPlain)
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
