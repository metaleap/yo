package yoauth

import (
	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	q "yo/db/query"
	. "yo/srv"
	. "yo/util"
	"yo/util/str"
)

const (
	CtxKeyEmailAddr = "yoUserEmailAddr"
	CtxKeyAccountId = "yoUserAccountId"

	MethodPathLoginOrFinalizePwdReset = "__/yo/authLoginOrFinalizePwdReset"
	MethodPathLogout                  = "__/yo/authLogout"
	MethodPathRegister                = "__/yo/authRegister"
	MethodPathChangePassword          = "__/yo/authChangePassword"
)

var (
	IsEmailishEnough = q.Via(str.IsEmailishEnough)
)

func init() {
	if IsDevMode {
		Apis(ApiMethods{
			MethodPathLogout: api(ApiUserLogout),

			MethodPathRegister: api(ApiUserRegister,
				Fails{Err: "EmailRequiredButMissing", If: ___yo_authRegisterEmailAddr.Equal("")},
				Fails{Err: "EmailInvalid", If: IsEmailishEnough(___yo_authRegisterEmailAddr).Not()},
				Fails{Err: "PasswordTooShort", If: ___yo_authRegisterPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
				Fails{Err: "PasswordTooLong", If: ___yo_authRegisterPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
			).
				CouldFailWith("EmailAddrAlreadyExists"),

			MethodPathLoginOrFinalizePwdReset: api(ApiUserLoginOrFinalizePwdReset,
				Fails{Err: "EmailRequiredButMissing", If: ___yo_authLoginOrFinalizePwdResetEmailAddr.Equal("")},
				Fails{Err: "EmailInvalid", If: IsEmailishEnough(___yo_authLoginOrFinalizePwdResetEmailAddr).Not()},
				Fails{Err: "WrongPassword",
					If: ___yo_authLoginOrFinalizePwdResetPasswordPlain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN).Or(
						___yo_authLoginOrFinalizePwdResetPasswordPlain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)).Or(
						___yo_authLoginOrFinalizePwdResetPasswordPlain.StrLen().GreaterThan(0).And(
							___yo_authLoginOrFinalizePwdResetPassword2Plain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN).Or(
								___yo_authLoginOrFinalizePwdResetPassword2Plain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN))))},
				Fails{Err: "NewPasswordTooShort", If: ___yo_authLoginOrFinalizePwdResetPassword2Plain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
				Fails{Err: "NewPasswordTooLong", If: ___yo_authLoginOrFinalizePwdResetPassword2Plain.StrLen().GreaterThan(Cfg.YO_AUTH_PWD_MAX_LEN)},
				Fails{Err: "NewPasswordExpectedToDiffer", If: ___yo_authLoginOrFinalizePwdResetPassword2Plain.Equal(___yo_authLoginOrFinalizePwdResetPasswordPlain)},
			).
				CouldFailWith(":"+yodb.ErrSetDbUpdate, "PwdReqExpired", "PwdResetRequired", "AccountDoesNotExist", ErrUnauthorized),

			MethodPathChangePassword: api(apiChangePassword,
				Fails{Err: "NewPasswordExpectedToDiffer", If: ___yo_authChangePasswordPassword2Plain.Equal(___yo_authChangePasswordPasswordPlain)},
				Fails{Err: "NewPasswordTooShort", If: ___yo_authChangePasswordPassword2Plain.StrLen().LessThan(Cfg.YO_AUTH_PWD_MIN_LEN)},
			).
				CouldFailWith(":" + MethodPathLoginOrFinalizePwdReset),
		})
	}

	PreServes = append(PreServes, Middleware{Name: "authCheck", Do: func(ctx *Ctx) {
		jwt_raw, forced_test_user := ctx.HttpGetCookie(Cfg.YO_AUTH_JWT_COOKIE_NAME), ctx.GetStr(CtxKeyForcedTestUser)
		if IsDevMode && (forced_test_user != "") {
			if cur_user_email_addr, cur_account_id := httpUserFromJwtRaw(jwt_raw); (cur_account_id == 0) ||
				(cur_user_email_addr != forced_test_user) {
				Do(ApiUserLoginOrFinalizePwdReset, ctx, &ApiAccountPayload{EmailAddr: forced_test_user, PasswordPlain: "foobar"})
				return
			}
		}
		httpSetUser(ctx, jwt_raw, false)
	}})
}

type ApiAccountPayload struct {
	EmailAddr      string
	PasswordPlain  string
	Password2Plain string
}

type ApiTokenPayload struct {
	JwtSignedToken string
}

func ApiUserRegister(this *ApiCtx[ApiAccountPayload, struct {
	Id yodb.I64
}]) {
	httpSetUser(this.Ctx, "", true)
	this.Ret.Id = UserRegister(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain)
}

func ApiUserLoginOrFinalizePwdReset(this *ApiCtx[ApiAccountPayload, UserAccount]) {
	if this.Args.Password2Plain != "" {
		if user_email_addr, _ := CurrentlyLoggedInUser(this.Ctx); (user_email_addr != "") && (user_email_addr != this.Args.EmailAddr) {
			panic(ErrUnauthorized)
		}
	}
	httpSetUser(this.Ctx, "", true)
	account, jwt_token := UserLoginOrFinalizeRegisterOrPwdReset(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain, this.Args.Password2Plain)
	if this.Ret = account; (account != nil) && (jwt_token != nil) {
		jwt_signed, err := jwt_token.SignedString([]byte(Cfg.YO_AUTH_JWT_SIGN_KEY))
		if err != nil {
			panic(err)
		}
		httpSetUser(this.Ctx, jwt_signed, true)
	}
}

func ApiUserLogout(ctx *ApiCtx[None, None]) {
	httpSetUser(ctx.Ctx, "", true)
}

func apiChangePassword(this *ApiCtx[ApiAccountPayload, None]) {
	user_email_addr, _ := CurrentlyLoggedInUser(this.Ctx)
	if user_email_addr != this.Args.EmailAddr {
		panic(ErrUnauthorized)
	}
	httpSetUser(this.Ctx, "", true)
	UserChangePassword(this.Ctx, this.Args.EmailAddr, this.Args.PasswordPlain, this.Args.Password2Plain)
}

func httpUserFromJwtRaw(jwtRaw string) (userEmailAddr string, UserAccountId yodb.I64) {
	if jwtRaw != "" {
		jwt_payload := UserVerify(jwtRaw)
		if jwt_payload != nil {
			userEmailAddr, UserAccountId = jwt_payload.StandardClaims.Subject, jwt_payload.UserAccountId
		}
	}
	return
}

func httpSetUser(ctx *Ctx, jwtRaw string, knownToExist bool) {
	user_email_addr, account_id := httpUserFromJwtRaw(jwtRaw)
	if user_email_addr = str.Trim(user_email_addr); (account_id <= 0) || (user_email_addr == "") || ((!knownToExist) && !yodb.Exists[UserAccount](ctx, UserAccountId.Equal(account_id).And(UserAccountEmailAddr.Equal(user_email_addr)))) {
		account_id, user_email_addr = 0, ""
		jwtRaw = ""
	}
	ctx.Set(CtxKeyAccountId, account_id)
	ctx.Set(CtxKeyEmailAddr, user_email_addr)
	ctx.Http.Resp.Header().Set(HttpResponseHeaderName_UserEmailAddr, user_email_addr)
	ctx.HttpSetCookie(Cfg.YO_AUTH_JWT_COOKIE_NAME, jwtRaw, Cfg.YO_AUTH_JWT_COOKIE_EXPIRY_DAYS)
	if IsDevMode && (Cfg.YO_AUTH_JWT_COOKIE_EXPIRY_DAYS > 400) {
		panic("illegal YO_AUTH_JWT_COOKIE_EXPIRY_DAYS for modern-browser cookie laws")
	}
}

func CurrentlyLoggedInUser(ctx *Ctx) (emailAddr string, authID yodb.I64) {
	return ctx.GetStr(CtxKeyEmailAddr), ctx.Get(CtxKeyAccountId, yodb.I64(0)).(yodb.I64)
}

func IsCurrentlyLoggedIn(ctx *Ctx) bool {
	user_email_addr, account_id := CurrentlyLoggedInUser(ctx)
	return (user_email_addr != "") && (account_id > 0)
}

func IsNotCurrentlyLoggedIn(ctx *Ctx) bool { return !IsCurrentlyLoggedIn(ctx) }
