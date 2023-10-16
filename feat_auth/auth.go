package yoauth

import (
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("my_secret_key")

type JwtPayload struct {
	jwt.StandardClaims
}

type UserAuth struct {
	Id      yodb.I64
	Created *yodb.DateTime

	EmailAddr      yodb.Text
	passwordHashed yodb.Bytes
}

func init() {
	yodb.Ensure[UserAuth, UserAuthField]("", nil)
}

func UserRegister(ctx *Ctx, emailAddr string, passwordPlain string) (ret yodb.I64) {
	ctx.DbTx()
	if yodb.Exists[UserAuth](ctx, UserAuthColEmailAddr.Equal(emailAddr)) {
		panic(Err___admin_authRegister_EmailAddrAlreadyExists)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(Err___admin_authRegister_PasswordTooLong)
		} else {
			panic(Err___admin_authRegister_PasswordInvalid)
		}
	}
	if ret = yodb.I64(yodb.CreateOne[UserAuth](ctx, &UserAuth{
		EmailAddr:      yodb.Text(emailAddr),
		passwordHashed: hash,
	})); ret <= 0 {
		panic(ErrDbNotStored)
	}
	return
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (*UserAuth, *jwt.Token) {
	user_account := yodb.FindOne[UserAuth](ctx, UserAuthColEmailAddr.Equal(emailAddr))
	if user_account == nil {
		panic(Err___admin_authLogin_AccountDoesNotExist)
	}

	err := bcrypt.CompareHashAndPassword(user_account.passwordHashed, []byte(passwordPlain))
	if err != nil {
		panic(Err___admin_authLogin_WrongPassword)
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

func UserChangePassword(ctx *Ctx, emailAddr string, passwordOldPlain string, passwordNewPlain string) {
	ctx.DbTx()
	user_account, _ := UserLogin(ctx, emailAddr, passwordOldPlain)
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordNewPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(Err___admin_authChangePassword_NewPasswordTooLong)
		} else {
			panic(Err___admin_authChangePassword_NewPasswordInvalid)
		}
	}
	user_account.passwordHashed = hash
	if (yodb.Update[UserAuth](ctx, user_account, false, nil)) < 1 {
		panic(ErrDbNotStored)
	}
}
