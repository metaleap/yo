package yofeat_auth

import (
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	. "yo/util"
	"yo/util/str"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("my_secret_key")

type JwtPayload struct {
	jwt.StandardClaims
}

type UserAccount struct {
	Id      yodb.I64
	Created yodb.DateTime

	EmailAddr      yodb.Text
	passwordHashed yodb.Bytes
}

func init() {
	yodb.Ensure[UserAccount, UserAccountField](false, "", nil)
}

func UserRegister(ctx *Ctx, emailAddr string, passwordPlain string) yodb.I64 {
	if emailAddr == "" {
		panic(Err("UserRegisterEmailRequiredButMissing"))
	}
	if !str.IsEmailishEnough(emailAddr) {
		panic(Err("UserRegisterEmailInvalid"))
	}
	if passwordPlain == "" {
		panic(Err("UserRegisterPasswordRequiredButMissing"))
	}
	if len(passwordPlain) < 6 {
		panic(Err("UserRegisterPasswordTooShort"))
	}
	ctx.DbTx(yodb.DB)
	if yodb.Exists[UserAccount](ctx, UserAccountColEmailAddr.Equal(emailAddr)) {
		panic(Err("UserRegisterEmailAddrAlreadyExists"))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(Err("UserRegisterPasswordTooLong"))
		} else {
			panic(Err("UserRegisterPasswordInvalid"))
		}
	}
	return yodb.CreateOne[UserAccount](ctx, &UserAccount{
		EmailAddr:      yodb.Text(emailAddr),
		passwordHashed: hash,
	})
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (jwtSignedToken string) {
	if emailAddr == "" {
		panic(Err("UserLoginEmailRequiredButMissing"))
	}
	if !str.IsEmailishEnough(emailAddr) { // saves a DB hit I guess =)
		panic(Err("UserLoginEmailInvalid"))
	}
	if passwordPlain == "" {
		panic(Err("UserLoginPasswordRequiredButMissing"))
	}
	user_account := yodb.FindOne[UserAccount](ctx, UserAccountColEmailAddr.Equal(emailAddr))
	if user_account == nil {
		panic(Err("UserLoginAccountDoesNotExist"))
	}

	err := bcrypt.CompareHashAndPassword(user_account.passwordHashed, []byte(passwordPlain))
	if err != nil {
		panic(Err("UserLoginPasswordInvalid"))
	}

	claims := &JwtPayload{
		StandardClaims: jwt.StandardClaims{
			Subject:   string(user_account.EmailAddr),
			ExpiresAt: time.Now().UTC().AddDate(0, 0, Cfg.YO_AUTH_JWT_EXPIRY_DAYS).Unix(),
		},
	}
	jwtSignedToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtKey)
	if err != nil {
		panic(Err("UserLoginOkButFailedToCreateSignedToken"))
	}
	return
}
