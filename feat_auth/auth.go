package yoauth

import (
	"crypto/rand"
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type JwtPayload struct {
	jwt.StandardClaims
	UserAuthId yodb.I64
}

type UserAuth struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	EmailAddr    yodb.Text
	PwdForgotten yodb.Bool
	pwdHashed    yodb.Bytes
}

func init() {
	yodb.Ensure[UserAuth, UserAuthField]("", nil, false,
		yodb.Unique[UserAuthField]{UserAuthEmailAddr})
}

func UserPregisterOrForgotPassword(ctx *Ctx, emailAddr string) {
	existing_user := yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
	if existing_user != nil {
		yodb.Update[UserAuth](ctx, &UserAuth{PwdForgotten: true}, UserAuthId.Equal(existing_user.Id), true, UserAuthFields(UserAuthPwdForgotten)...)
	} else {
		random_bytes := make([]byte, 128)
		_, _ = rand.Read(random_bytes)
		UserRegister(ctx, emailAddr, string(random_bytes), true)
	}
}

func UserRegister(ctx *Ctx, emailAddr string, passwordPlain string, pwdForgotten yodb.Bool) yodb.I64 {
	ctx.DbTx()

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
		EmailAddr:    yodb.Text(emailAddr),
		pwdHashed:    pwd_hashed,
		PwdForgotten: pwdForgotten,
	}))
}

func UserLogin(ctx *Ctx, emailAddr string, passwordPlain string) (*UserAuth, *jwt.Token) {
	user_auth := yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(emailAddr))
	if user_auth == nil {
		panic(Err___yo_authLogin_AccountDoesNotExist)
	}

	err := bcrypt.CompareHashAndPassword(user_auth.pwdHashed, []byte(passwordPlain))
	if err != nil {
		panic(Err___yo_authLogin_WrongPassword)
	}

	return user_auth, jwt.NewWithClaims(jwt.SigningMethodHS256, &JwtPayload{
		UserAuthId: user_auth.Id,
		StandardClaims: jwt.StandardClaims{
			Subject:   string(user_auth.EmailAddr),
			ExpiresAt: time.Now().UTC().AddDate(0, 0, Cfg.YO_AUTH_JWT_EXPIRY_DAYS).Unix(),
		},
	})
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
	ctx.DbTx()
	user_account, _ := UserLogin(ctx, emailAddr, passwordOldPlain)
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordNewPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		if err == bcrypt.ErrPasswordTooLong {
			panic(Err___yo_authChangePassword_NewPasswordTooLong)
		} else {
			panic(Err___yo_authChangePassword_NewPasswordInvalid)
		}
	}
	user_account.pwdHashed = hash
	_ = yodb.Update[UserAuth](ctx, user_account, nil, true, userAuthPwdHashed.F())
}

func ById(ctx *Ctx, id yodb.I64) *UserAuth {
	return yodb.FindOne[UserAuth](ctx, UserAuthId.Equal(id))
}
