package yoauth

import (
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	yomail "yo/mail"
	. "yo/util"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

var (
	AutoLoginAfterSuccessfullyFinalizedSignUpOrPwdResetReq = false
	EnforceGenericErrors                                   = true
)

const errGeneric = Err("InvalidCredentials")

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

func Init() {
	if (!IsDevMode) && EnforceGenericErrors {
		ErrReplacements[errGeneric] = []Err{
			Err___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist, Err___yo_authLoginOrFinalizePwdReset_NewPasswordExpectedToDiffer, Err___yo_authLoginOrFinalizePwdReset_OkButFailedToCreateSignedToken, Err___yo_authLoginOrFinalizePwdReset_WrongPassword,
			Err___yo_authRegister_EmailAddrAlreadyExists,
		}
	}
}

func UserPregisterOrForgotPassword(ctx *Ctx, emailAddr string) {
	yodb.Upsert[UserPwdReq](ctx, &UserPwdReq{EmailAddr: yodb.Text(emailAddr)})
}

func UserRegister(ctx *Ctx, emailAddr string, passwordPlain string) (ret yodb.I64) {
	ctx.DbTx(true)

	if IsDevMode && yodb.Exists[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr)) {
		// this branch never taken in prod to help prevent time-based-attacks.
		// the DB-side unique constraint will still fail the insert attempt (and genericized in prod, see below).
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
	Try(func() {
		ret = yodb.I64(yodb.CreateOne[UserAuth](ctx, &UserAuth{
			EmailAddr: yodb.Text(emailAddr),
			pwdHashed: pwd_hashed,
		}))
	}, func(err any) {
		panic(If[any](IsDevMode, err, errGeneric))
	})
	return
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (okUserAuth *UserAuth, okJwt *jwt.Token) {
	user_auth := yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
	if IsDevMode && user_auth == nil { // not in prod, to guard against time-based-attacks. so do the pwd-hash-check even with no-such-user
		panic(Err___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist)
	}

	user_email_addr, user_auth_id, user_pwd_hashed := "nosuchuser@never.com", yodb.I64(0), []byte(newRandomAsciiOneTimePwd(60)) // for the same reason, we do this in-any-case, even though:
	if user_auth != nil {
		user_email_addr, user_auth_id, user_pwd_hashed = user_auth.EmailAddr.String(), user_auth.Id, user_auth.pwdHashed
	}

	err := bcrypt.CompareHashAndPassword(user_pwd_hashed, []byte(passwordPlain))
	okUserAuth, okJwt = user_auth, jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtPayload{
		UserAuthId: user_auth_id,
		StandardClaims: jwt.StandardClaims{
			Subject: string(user_email_addr),
		},
	})
	if err != nil {
		okUserAuth, okJwt = nil, nil // looks pointless here, but... maybe it ain't  =)
		panic(Err___yo_authLoginOrFinalizePwdReset_WrongPassword)
	}
	return
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
	if (token != nil) && token.Valid && (token.Claims != nil) {
		if payload, _ := token.Claims.(*JwtPayload); (payload != nil) && (payload.Subject != "") {
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
