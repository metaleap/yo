package yoauth

import (
	"time"
	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	yomail "yo/mail"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

var AutoLoginAfterSuccessfullyFinalizedSignUpOrPwdResetReq = false

type JwtPayload struct {
	jwt.StandardClaims
	UserAuthId yodb.I64
}

type UserAuth struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	EmailAddr yodb.Text
	pwdHashed yodb.Bytes
}

type UserPwdReq struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	EmailAddr     yodb.Text
	DoneMailReqId yodb.Ref[yomail.MailReq, yodb.RefOnDelCascade]
	DtFinalized   *yodb.DateTime
	tmpPwdHashed  yodb.Bytes
}

func init() {
	yodb.Ensure[UserAuth, UserAuthField]("", nil, false,
		yodb.Index[UserAuthField]{UserAuthEmailAddr},
		yodb.Unique[UserAuthField]{UserAuthEmailAddr})
	yodb.Ensure[UserPwdReq, UserPwdReqField]("", nil, false,
		yodb.Index[UserPwdReqField]{UserPwdReqEmailAddr},
		yodb.Unique[UserPwdReqField]{UserPwdReqEmailAddr})
}

func UserPregisterOrForgotPassword(ctx *Ctx, emailAddr string) {
	yodb.Upsert[UserPwdReq](ctx, &UserPwdReq{EmailAddr: yodb.Text(emailAddr)})
}

func UserRegister(ctx *Ctx, emailAddr string, passwordPlain string) yodb.I64 {
	ctx.DbTx(true)

	if yodb.Exists[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr)) {
		panic(Err___yo_authRegister_EmailAddrAlreadyExists)
	}
	pwd_hashed, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(pwd_hashed) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(Err___yo_authRegister_PasswordTooLong)
		} else {
			panic(Err___yo_authRegister_PasswordInvalid)
		}
	}
	return yodb.I64(yodb.CreateOne[UserAuth](ctx, &UserAuth{
		EmailAddr: yodb.Text(emailAddr),
		pwdHashed: pwd_hashed,
	}))
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (*UserAuth, *jwt.Token) {
	user_auth := yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
	if user_auth == nil {
		panic(Err___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist)
	}

	err := bcrypt.CompareHashAndPassword(user_auth.pwdHashed, []byte(passwordPlain))
	if err != nil {
		panic(Err___yo_authLoginOrFinalizePwdReset_WrongPassword)
	}
	return user_auth, jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtPayload{
		UserAuthId: user_auth.Id,
		StandardClaims: jwt.StandardClaims{
			Subject: string(user_auth.EmailAddr),
		},
	})
}

func UserLoginOrFinalizeRegisterOrPwdReset(ctx *Ctx, emailAddr string, passwordPlain string, password2Plain string) (*UserAuth, *jwt.Token) {
	if password2Plain == "" {
		return UserLogin(ctx, emailAddr, passwordPlain)
	}

	pwd_reset_req := yodb.FindOne[UserPwdReq](ctx,
		UserPwdReqEmailAddr.Equal(emailAddr). // request for this email addr
							And(UserPwdReqDoneMailReqId.NotEqual(nil)).        // where the corresponding mail-req was already created
							And(userPwdReqDoneMailReqId_dtDone.NotEqual(nil)). // and its mail also sent out successfully
							And(userPwdReqTmpPwdHashed.NotEqual(nil)).         // hence the temp one-time pwd is still there
							And(UserPwdReqDtFinalized.Equal(nil)))             // because that pwd-req (sign-up or pwd-reset) wasn't finalized yet (which happens in here, below)

	if pwd_reset_req == nil {
		UserChangePassword(ctx, emailAddr, passwordPlain, password2Plain)
		return UserLogin(ctx, emailAddr, password2Plain)
	}

	if (Cfg.YO_AUTH_PWD_REQ_VALIDITY_MINS > 0) && ((int(time.Now().Sub(*pwd_reset_req.DtMade.Time()).Minutes())) > (Cfg.YO_AUTH_PWD_REQ_VALIDITY_MINS + /* some leeway for mail-req job-task & mail delivery */ 2)) {
		yodb.Delete[UserPwdReq](ctx, yodb.ColID.Equal(pwd_reset_req.Id))
		panic(Err___yo_authLoginOrFinalizePwdReset_PwdReqExpired)
	}
	// check temp one-time pwd
	err := bcrypt.CompareHashAndPassword(pwd_reset_req.tmpPwdHashed, []byte(passwordPlain))
	if err != nil {
		panic(Err___yo_authLoginOrFinalizePwdReset_WrongPassword)
	}
	pwd_hash, err := bcrypt.GenerateFromPassword([]byte(password2Plain), bcrypt.DefaultCost)
	if (err != nil) || (len(pwd_hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(Err___yo_authLoginOrFinalizePwdReset_NewPasswordTooLong)
		} else {
			panic(Err___yo_authLoginOrFinalizePwdReset_NewPasswordInvalid)
		}
	}
	ctx.DbTx(true)
	user_auth := yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
	if user_auth != nil { // existing user: pwd-reset
		user_auth.pwdHashed = pwd_hash
		_ = yodb.Update[UserAuth](ctx, user_auth, nil, true, userAuthPwdHashed.F())
	} else { // new user: register
		user_auth = &UserAuth{pwdHashed: pwd_hash, EmailAddr: yodb.Text(emailAddr)}
		_ = yodb.CreateOne[UserAuth](ctx, user_auth)
	}
	pwd_reset_req.tmpPwdHashed, pwd_reset_req.DtFinalized = nil, yodb.DtNow()
	if yodb.Update(ctx, pwd_reset_req, nil, false, UserPwdReqFields(userPwdReqTmpPwdHashed, UserPwdReqDtFinalized)...) < 0 {
		panic(ErrDbUpdate_ExpectedChangesForUpdate)
	}
	if AutoLoginAfterSuccessfullyFinalizedSignUpOrPwdResetReq {
		login_user_auth, login_jwt_token := UserLogin(ctx, emailAddr, password2Plain)
		return login_user_auth, login_jwt_token
	}

	return nil, nil
}

func UserVerify(jwtRaw string) *JwtPayload {
	token, _ := jwt.ParseWithClaims(jwtRaw, &JwtPayload{}, func(token *jwt.Token) (any, error) {
		return []byte(Cfg.YO_AUTH_JWT_SIGN_KEY), nil
	})
	if (token != nil) && (token.Claims != nil) {
		if payload, is := token.Claims.(*JwtPayload); is && (payload.Subject != "") {
			return payload
		}
	}
	return nil
}

func UserChangePassword(ctx *Ctx, emailAddr string, passwordOldPlain string, passwordNewPlain string) {
	ctx.DbTx(true)
	user_account, _ := UserLogin(ctx, emailAddr, passwordOldPlain)
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordNewPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(Err___yo_authLoginOrFinalizePwdReset_NewPasswordTooLong)
		} else {
			panic(Err___yo_authLoginOrFinalizePwdReset_NewPasswordInvalid)
		}
	}
	user_account.pwdHashed = hash
	_ = yodb.Update[UserAuth](ctx, user_account, nil, true, userAuthPwdHashed.F())
}

func ById(ctx *Ctx, id yodb.I64) *UserAuth {
	return yodb.ById[UserAuth](ctx, id)
}

func ByEmailAddr(ctx *Ctx, emailAddr string) *UserAuth {
	return yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
}
