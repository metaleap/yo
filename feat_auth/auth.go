package yoauth

import (
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
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
		panic(ErrAuthRegisterEmailRequiredButMissing)
	}
	if !str.IsEmailishEnough(emailAddr) {
		panic(ErrAuthRegisterEmailInvalid)
	}
	if passwordPlain == "" {
		panic(ErrAuthRegisterPasswordRequiredButMissing)
	}
	if len(passwordPlain) < 6 {
		panic(ErrAuthRegisterPasswordTooShort)
	}
	ctx.DbTx()
	if yodb.Exists[UserAccount](ctx, UserAccountColEmailAddr.Equal(emailAddr)) {
		panic(ErrAuthRegisterEmailAddrAlreadyExists)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(ErrAuthRegisterPasswordTooLong)
		} else {
			panic(ErrAuthRegisterPasswordInvalid)
		}
	}
	return yodb.CreateOne[UserAccount](ctx, &UserAccount{
		EmailAddr:      yodb.Text(emailAddr),
		passwordHashed: hash,
	})
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (*UserAccount, *jwt.Token) {
	if emailAddr == "" {
		panic(ErrAuthLoginEmailRequiredButMissing)
	}
	if !str.IsEmailishEnough(emailAddr) { // saves a DB hit I guess =)
		panic(ErrAuthLoginEmailInvalid)
	}
	if passwordPlain == "" {
		panic(ErrAuthLoginPasswordRequiredButMissing)
	}
	user_account := yodb.FindOne[UserAccount](ctx, UserAccountColEmailAddr.Equal(emailAddr))
	if user_account == nil {
		panic(ErrAuthLoginAccountDoesNotExist)
	}

	err := bcrypt.CompareHashAndPassword(user_account.passwordHashed, []byte(passwordPlain))
	if err != nil {
		panic(ErrAuthLoginWrongPassword)
	}

	return user_account, jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtPayload{
		StandardClaims: jwt.StandardClaims{
			Subject:   string(user_account.EmailAddr),
			ExpiresAt: time.Now().UTC().AddDate(0, 0, Cfg.YO_AUTH_JWT_EXPIRY_DAYS).Unix(),
		},
	})
}

func UserVerify(ctx *Ctx, jwtRaw string) *JwtPayload {
	token, _ := jwt.ParseWithClaims(jwtRaw, &JwtPayload{}, func(token *jwt.Token) (any, error) {
		return []byte(jwtKey), nil
	})
	if (token != nil) && (token.Claims != nil) {
		if payload, is := token.Claims.(*JwtPayload); is && (payload.Subject != "") {
			return payload
		}
	}
	return nil
}

func UserChangePassword(ctx *Ctx, emailAddr string, passwordOldPlain string, passwordNewPlain string) bool {
	if passwordNewPlain == "" {
		panic(ErrAuthChangePasswordNewPasswordRequiredButMissing)
	}
	if len(passwordNewPlain) < 6 {
		panic(ErrAuthChangePasswordNewPasswordTooShort)
	}
	if passwordNewPlain == passwordOldPlain {
		panic(ErrAuthChangePasswordNewPasswordSameAsOld)
	}
	ctx.DbTx()
	user_account, _ := UserLogin(ctx, emailAddr, passwordOldPlain)
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordNewPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(ErrAuthChangePasswordNewPasswordTooLong)
		} else {
			panic(ErrAuthChangePasswordNewPasswordInvalid)
		}
	}
	user_account.passwordHashed = hash
	return (yodb.Update[UserAccount](ctx, user_account, false, nil)) > 0
}
