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
	CtxKeyEmailAddr   = "yoUserEmailAddr"
	CtxKeyAuthId      = "yoUserAuthId"
	HttpUserHeader    = "X-Yo-User"
	HttpJwtCookieName = "t"

	MethodPathLogin          = "__/yo/authLogin"
	MethodPathLogout         = "__/yo/authLogout"
	MethodPathRegister       = "__/yo/authRegister"
	MethodPathChangePassword = "__/yo/authChangePassword"
)

var (
	IsEmailishEnough = q.Via(str.IsEmailishEnough)
)

func init() {
	Apis(ApiMethods{
		MethodPathLogout: api(ApiUserLogout),

		MethodPathLogin: api(ApiUserLogin,
			Fails{Err: "EmailRequiredButMissing", If: ___yo_authRegisterEmailAddr.Equal("")},
			Fails{Err: "EmailInvalid", If: IsEmailishEnough(___yo_authLoginEmailAddr).Not()},
			Fails{Err: "WrongPassword",
				If: ___yo_authLoginPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN).Or(
					___yo_authLoginPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN))},
		).
			CouldFailWith("OkButFailedToCreateSignedToken", "AccountDoesNotExist"),

		MethodPathRegister: api(ApiUserRegister,
			Fails{Err: "EmailRequiredButMissing", If: ___yo_authRegisterEmailAddr.Equal("")},
			Fails{Err: "EmailInvalid", If: IsEmailishEnough(___yo_authRegisterEmailAddr).Not()},
			Fails{Err: "PasswordTooShort", If: ___yo_authRegisterPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
			Fails{Err: "PasswordTooLong", If: ___yo_authRegisterPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
		).
			CouldFailWith("EmailAddrAlreadyExists", "PasswordInvalid"),

		MethodPathChangePassword: api(apiChangePassword,
			Fails{Err: "NewPasswordExpectedToDiffer", If: ___yo_authChangePasswordPasswordNewPlain.Equal(___yo_authChangePasswordPasswordPlain)},
			Fails{Err: "NewPasswordTooShort", If: ___yo_authChangePasswordPasswordNewPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
			Fails{Err: "NewPasswordTooLong", If: ___yo_authChangePasswordPasswordNewPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
		).
			CouldFailWith(":"+yodb.ErrSetDbUpdate, ":"+MethodPathLogin, "NewPasswordInvalid", ErrUnauthorized),
	})

	PreServes = append(PreServes, Middleware{Name: "authCheck", Do: func(ctx *Ctx) {
		jwt_raw, forced_test_user := ctx.HttpGetCookie(HttpJwtCookieName), ctx.GetStr(CtxKeyForcedTestUser)
		if IsDevMode && (forced_test_user != "") {
			if cur_user_email_addr, cur_user_auth_id := httpUserFromJwtRaw(jwt_raw); (cur_user_auth_id == 0) ||
				(cur_user_email_addr != forced_test_user) {
				Do(ApiUserLogin, ctx, &ApiAccountPayload{EmailAddr: forced_test_user, PasswordPlain: "foobar"})
				return
			}
		}
		httpSetUser(ctx, jwt_raw)
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
	jwt_signed, err := jwt_token.SignedString([]byte(Cfg.YO_AUTH_JWT_SIGN_KEY))
	if err != nil {
		panic(Err___yo_authLogin_OkButFailedToCreateSignedToken)
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
	user_email_addr, _ := CurrentlyLoggedInUser(this.Ctx)
	if user_email_addr != this.Args.EmailAddr {
		panic(ErrUnauthorized)
	}
	httpSetUser(this.Ctx, "")
	UserChangePassword(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain, this.Args.PasswordNewPlain)
}

func httpUserFromJwtRaw(jwtRaw string) (userEmailAddr string, userAuthId yodb.I64) {
	if jwtRaw != "" {
		if jwt_payload := UserVerify(jwtRaw); jwt_payload != nil {
			userEmailAddr, userAuthId = jwt_payload.StandardClaims.Subject, jwt_payload.UserAuthId
		}
	}
	return
}

func httpSetUser(ctx *Ctx, jwtRaw string) {
	user_email_addr, user_auth_id := httpUserFromJwtRaw(jwtRaw)
	ctx.Set(CtxKeyEmailAddr, user_email_addr)
	ctx.Set(CtxKeyAuthId, user_auth_id)
	ctx.Http.Resp.Header().Set(HttpUserHeader, user_email_addr)
	ctx.HttpSetCookie(HttpJwtCookieName, jwtRaw, Cfg.YO_AUTH_JWT_EXPIRY_DAYS)
}

func CurrentlyLoggedInUser(ctx *Ctx) (emailAddr string, authID yodb.I64) {
	return ctx.GetStr(CtxKeyEmailAddr), ctx.Get(CtxKeyAuthId, yodb.I64(0)).(yodb.I64)
}

func CurrentlyLoggedIn(ctx *Ctx) bool {
	user_email_addr, user_auth_id := CurrentlyLoggedInUser(ctx)
	return (user_email_addr != "") && (user_auth_id > 0)
}

func CurrentlyNotLoggedIn(ctx *Ctx) bool { return !CurrentlyLoggedIn(ctx) }
