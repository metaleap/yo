package yoauth

import (
	"crypto/sha512"
	"sort"
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
	UserAccountId yodb.I64
}

type UserAccount struct {
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
	yodb.Ensure[UserAccount, UserAccountField]("", nil, false,
		yodb.Index[UserAccountField]{UserAccountEmailAddr},
		yodb.Unique[UserAccountField]{UserAccountEmailAddr})
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

	if IsDevMode && yodb.Exists[UserAccount](ctx, UserAccountEmailAddr.Equal(emailAddr)) {
		// this branch never taken in prod to help prevent time-based-attacks.
		// the DB-side unique constraint will still fail the insert attempt (and genericized in prod, see below).
		panic(Err___yo_authRegister_EmailAddrAlreadyExists)
	}

	Try(func() {
		ret = yodb.I64(yodb.CreateOne[UserAccount](ctx, &UserAccount{
			EmailAddr: yodb.Text(emailAddr),
			pwdHashed: pwdHashStorable(passwordPlain, emailAddr),
		}))
	}, func(err any) {
		panic(If[any](EnforceGenericizedErrors, err, errGeneric))
	})
	return
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (*UserAccount, *jwt.Token) {
	account := yodb.FindOne[UserAccount](ctx, UserAccountEmailAddr.Equal(emailAddr))
	if IsDevMode && account == nil { // not in prod, to guard against time-based-attacks. so do the pwd-hash-check even with no-such-user
		panic(Err___yo_authLoginOrFinalizePwdReset_AccountDoesNotExist)
	}
	if (account != nil) && account.Lockout { // at this point, no more obscuration needed (afaik=)
		panic(Err___yo_authLoginOrFinalizePwdReset_PwdResetRequired)
	}

	user_email_addr, account_id, user_pwd_hashed := "nosuchuser@never.com", yodb.I64(0), []byte(str.AsciiRand(60, 0)) // for the same reason, we do this in-any-case, even though:
	if account != nil {
		user_email_addr, account_id, user_pwd_hashed = account.EmailAddr.String(), account.Id, account.pwdHashed
	}

	if !pwdHashVerify(user_pwd_hashed, passwordPlain, user_email_addr) {
		if (account != nil) && (!account.Lockout) && (LoginThrottling.NumFailedAttemptsBeforeLockout > 0) && (LoginThrottling.WithinTimePeriod > 0) {
			account.FailedLoginAttempts = append(account.FailedLoginAttempts, yodb.I64(time.Now().UnixNano()))
			if idx_start := account.FailedLoginAttempts.Len() - LoginThrottling.NumFailedAttemptsBeforeLockout; idx_start >= 0 {
				last_n_attempts := account.FailedLoginAttempts[idx_start:]
				if Duration(sl.As(last_n_attempts, yodb.I64.Self)...) > LoginThrottling.WithinTimePeriod {
					account.Lockout = true
					UserPregisterOrForgotPassword(ctx, account.EmailAddr.String()) // (re)trigger pwd-reset-req mail
				}
			}
			yodb.Update[UserAccount](ctx, account, nil, false, UserAccountFields(UserAccountFailedLoginAttempts, UserAccountLockout)...)
		}
		panic(Err___yo_authLoginOrFinalizePwdReset_WrongPassword)
	}
	return account, jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtPayload{
		UserAccountId: account_id,
		StandardClaims: jwt.StandardClaims{
			Subject: string(user_email_addr),
		},
	})
}

func UserLoginOrFinalizeRegisterOrPwdReset(ctx *Ctx, emailAddr string, passwordPlain string, password2Plain string) (*UserAccount, *jwt.Token) {
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
	if !pwdHashVerify(pwd_reset_req.tmpPwdHashed, passwordPlain, emailAddr) {
		panic(Err___yo_authLoginOrFinalizePwdReset_WrongPassword)
	}
	pwd_hash := pwdHashStorable(password2Plain, emailAddr)
	ctx.DbTx(true)
	account := yodb.FindOne[UserAccount](ctx, UserAccountEmailAddr.Equal(emailAddr))
	if account != nil { // existing user: pwd-reset
		account.pwdHashed, account.FailedLoginAttempts, account.Lockout = pwd_hash, nil, false
		_ = yodb.Update[UserAccount](ctx, account, nil, true, UserAccountFields(userAccountPwdHashed, UserAccountFailedLoginAttempts, UserAccountLockout)...)
	} else { // new user: register
		account = &UserAccount{pwdHashed: pwd_hash, EmailAddr: yodb.Text(emailAddr)}
		account.Id = yodb.CreateOne[UserAccount](ctx, account)
	}
	pwd_reset_req.tmpPwdHashed = nil
	if yodb.Update(ctx, pwd_reset_req, nil, false, UserPwdReqFields(userPwdReqTmpPwdHashed)...) < 0 {
		panic(ErrDbUpdate_ExpectedChangesForUpdate)
	}
	if AutoLoginAfterSuccessfullyFinalizedSignUpOrPwdResetReq {
		return UserLogin(ctx, emailAddr, password2Plain)
	}

	return account, nil
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
	hash := pwdHashStorable(passwordNewPlain, emailAddr)
	user_account.pwdHashed, user_account.FailedLoginAttempts, user_account.Lockout = hash, nil, false
	_ = yodb.Update[UserAccount](ctx, user_account, nil, true, UserAccountFields(userAccountPwdHashed, UserAccountFailedLoginAttempts, UserAccountLockout)...)
}

func ById(ctx *Ctx, id yodb.I64) *UserAccount {
	return yodb.ById[UserAccount](ctx, id)
}

func ByEmailAddr(ctx *Ctx, emailAddr string) *UserAccount {
	return yodb.FindOne[UserAccount](ctx, UserAccountEmailAddr.Equal(emailAddr))
}

func pwdHashVerify(pwdHashStored []byte, passwordToCheckPlain string, salt string) bool {
	return (nil == bcrypt.CompareHashAndPassword(pwdHashStored, pwdPlainToBytes(passwordToCheckPlain, salt)))
}

func pwdPlainToBytes(passwordPlain string, salt string) []byte {
	sha512 := sha512.New()
	_, err := sha512.Write([]byte(passwordPlain))
	if err != nil {
		panic(err)
	}
	sha512hash := sha512.Sum(nil)
	Assert(len(sha512hash) == 64, len(sha512hash))

	Assert(len(salt) > 0, nil)
	bytes_salt := []byte(If((len(salt)%2) == 1, str.Lo, str.Up)(salt))
	for len(bytes_salt) < 8 {
		bytes_salt = append(bytes_salt, bytes_salt...)
	}
	sort.Slice(bytes_salt, func(i int, j int) bool {
		return (bytes_salt[i] < bytes_salt[j]) == ((len(salt) % 2) == 0)
	})
	bytes8 := bytes_salt[:8]
	if (len(salt) % 2) == 1 {
		bytes8 = bytes_salt[len(salt)-8:]
	}
	return append(sha512hash, bytes8[:]...)
}

func pwdHashStorable(passwordPlain string, salt string) []byte {
	ret, err := bcrypt.GenerateFromPassword(pwdPlainToBytes(passwordPlain, salt), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return ret
}
