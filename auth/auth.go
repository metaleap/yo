package yoauth

import (
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	yomail "yo/mail"
	. "yo/util"
	sl "yo/util/sl"
	"yo/util/str"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

var (
	AutoLoginAfterSuccessfullyFinalizedSignUpOrPwdResetReq = false
	EnforceGenericizedErrors                               = true
)

const errGeneric = Err("InvalidCredentials")

var LoginThrottling = struct {
	NumFailedAttemptsBeforeLockout int
	WithinTimePeriod               time.Duration
}{0, 0}

type JwtPayload struct {
	jwt.StandardClaims
	UserAuthId yodb.I64
}

type UserAuth struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	EmailAddr           yodb.Text
	pwdHashed           yodb.Bytes
	FailedLoginAttempts yodb.Arr[yodb.I64]
	Lockout             yodb.Bool
}

type UserPwdReq struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	EmailAddr     yodb.Text
	DoneMailReqId yodb.Ref[yomail.MailReq, yodb.RefOnDelCascade]
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
	if EnforceGenericizedErrors {
		ErrReplacements[errGeneric] = []Err{
			Err___yo_authRegister_EmailAddrAlreadyExists,
			Err___yo_authLoginOrFinalizePwdReset_PwdReqExpired, Err___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist, Err___yo_authLoginOrFinalizePwdReset_NewPasswordExpectedToDiffer, Err___yo_authLoginOrFinalizePwdReset_OkButFailedToCreateSignedToken, Err___yo_authLoginOrFinalizePwdReset_WrongPassword,
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
		panic(If[any](EnforceGenericizedErrors, err, errGeneric))
	})
	return
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (*UserAuth, *jwt.Token) {
	user_auth := yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
	if IsDevMode && user_auth == nil { // not in prod, to guard against time-based-attacks. so do the pwd-hash-check even with no-such-user
		panic(Err___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist)
	}
	if (user_auth != nil) && user_auth.Lockout { // at this point, no more obscuration needed (afaik=)
		panic(Err___yo_authLoginOrFinalizePwdReset_PwdResetRequired)
	}

	user_email_addr, user_auth_id, user_pwd_hashed := "nosuchuser@never.com", yodb.I64(0), []byte(str.AsciiRand(60, 0)) // for the same reason, we do this in-any-case, even though:
	if user_auth != nil {
		user_email_addr, user_auth_id, user_pwd_hashed = user_auth.EmailAddr.String(), user_auth.Id, user_auth.pwdHashed
	}

	err := bcrypt.CompareHashAndPassword(user_pwd_hashed, []byte(passwordPlain))
	if err != nil {
		if (user_auth != nil) && (!user_auth.Lockout) && (LoginThrottling.NumFailedAttemptsBeforeLockout > 0) && (LoginThrottling.WithinTimePeriod > 0) {
			user_auth.FailedLoginAttempts = append(user_auth.FailedLoginAttempts, yodb.I64(time.Now().UnixNano()))
			if idx_start := user_auth.FailedLoginAttempts.Len() - LoginThrottling.NumFailedAttemptsBeforeLockout; idx_start >= 0 {
				last_n_attempts := user_auth.FailedLoginAttempts[idx_start:]
				if Duration(sl.As(last_n_attempts, yodb.I64.Self)...) > LoginThrottling.WithinTimePeriod {
					user_auth.Lockout = true
					UserPregisterOrForgotPassword(ctx, user_auth.EmailAddr.String()) // (re)trigger pwd-reset-req mail
				}
			}
			yodb.Update[UserAuth](ctx, user_auth, nil, false, UserAuthFields(UserAuthFailedLoginAttempts, UserAuthLockout)...)
		}
		panic(Err___yo_authLoginOrFinalizePwdReset_WrongPassword)
	}
	return user_auth, jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtPayload{
		UserAuthId: user_auth_id,
		StandardClaims: jwt.StandardClaims{
			Subject: string(user_email_addr),
		},
	})
}

func UserLoginOrFinalizeRegisterOrPwdReset(ctx *Ctx, emailAddr string, passwordPlain string, password2Plain string) (*UserAuth, *jwt.Token) {
	if password2Plain == "" {
		return UserLogin(ctx, emailAddr, passwordPlain)
	}

	pwd_reset_req := yodb.FindOne[UserPwdReq](ctx,
		UserPwdReqEmailAddr.Equal(emailAddr). // request for this email addr..
							And(UserPwdReqDoneMailReqId.NotEqual(nil)).        // ..where the corresponding mail-req was already created
							And(userPwdReqDoneMailReqId_dtDone.NotEqual(nil)). // ..and its mail also sent out successfully
							And(userPwdReqTmpPwdHashed.NotEqual(nil)))         // ..hence the temp one-time pwd is still there

	if (pwd_reset_req == nil) || ((Cfg.YO_AUTH_PWD_REQ_VALIDITY_MINS > 0) &&
		((int(time.Now().Sub(*pwd_reset_req.DtMade.Time()).Minutes())) > (Cfg.YO_AUTH_PWD_REQ_VALIDITY_MINS + /* some leeway for mail-req job-task & mail delivery */ 2))) {
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
		user_auth.pwdHashed, user_auth.FailedLoginAttempts, user_auth.Lockout = pwd_hash, nil, false
		_ = yodb.Update[UserAuth](ctx, user_auth, nil, true, UserAuthFields(userAuthPwdHashed, UserAuthFailedLoginAttempts, UserAuthLockout)...)
	} else { // new user: register
		user_auth = &UserAuth{pwdHashed: pwd_hash, EmailAddr: yodb.Text(emailAddr)}
		user_auth.Id = yodb.CreateOne[UserAuth](ctx, user_auth)
	}
	pwd_reset_req.tmpPwdHashed = nil
	if yodb.Update(ctx, pwd_reset_req, nil, false, UserPwdReqFields(userPwdReqTmpPwdHashed)...) < 0 {
		panic(ErrDbUpdate_ExpectedChangesForUpdate)
	}
	if AutoLoginAfterSuccessfullyFinalizedSignUpOrPwdResetReq {
		return UserLogin(ctx, emailAddr, password2Plain)
	}

	return user_auth, nil
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
	user_account.pwdHashed, user_account.FailedLoginAttempts, user_account.Lockout = hash, nil, false
	_ = yodb.Update[UserAuth](ctx, user_account, nil, true, UserAuthFields(userAuthPwdHashed, UserAuthFailedLoginAttempts, UserAuthLockout)...)
}

func ById(ctx *Ctx, id yodb.I64) *UserAuth {
	return yodb.ById[UserAuth](ctx, id)
}

func ByEmailAddr(ctx *Ctx, emailAddr string) *UserAuth {
	return yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
}
