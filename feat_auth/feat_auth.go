package yofeat_auth

import (
	. "yo/ctx"
	yodb "yo/db"
	yoserve "yo/server"
	. "yo/util"

	"golang.org/x/crypto/bcrypt"
)

type UserAccount struct {
	Id      yodb.I64
	Created yodb.DateTime

	EmailAddr      yodb.Text
	passwordHashed yodb.Bytes
}

func init() {
	yodb.Ensure[UserAccount, UserAccountField](false, "", nil)
	yoserve.API["authRegister"] = yoserve.Method(apiUserRegister)
}

func UserRegister(ctx *Ctx, emailAddr string, passwordPlain string) yodb.I64 {
	if emailAddr == "" {
		panic(Err("UserRegisterEmailRequiredButMissing"))
	}
	if passwordPlain == "" {
		panic(Err("UserRegisterPasswordRequiredButMissing"))
	}
	ctx.DbTx(yodb.DB)
	if yodb.Exists[UserAccount](ctx, UserAccountColEmailAddr.Equal(emailAddr)) {
		panic(Err("UserRegisterEmailAddrAlreadyExists"))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(passwordPlain), bcrypt.DefaultCost)
	if (err != nil) || (len(hash) == 0) {
		panic(Err("PasswordInvalid"))
	}
	return yodb.CreateOne[UserAccount](ctx, &UserAccount{
		EmailAddr:      yodb.Text(emailAddr),
		passwordHashed: hash,
	})
}

func UserLogin(ctx *Ctx) {

	/*
	   loginEmail, loginPassword = strTrim(loginEmail), strTrim(loginPassword)

	   	if loginEmail == "" {
	   		return "", ErrorEmailRequiredButMissing
	   	}

	   	if loginPassword == "" {
	   		return "", ErrorPasswordRequiredButMissing
	   	}

	   user, err := dbGet[User](me, User{Email: loginEmail})

	   	if err != nil {
	   		return "", err
	   	} else if user == nil {

	   		return "", ErrorAuthUserDoesNotExist
	   	}

	   	if err = bcrypt.CompareHashAndPassword(user.PasswordHashed, []byte(loginPassword)); err != nil {
	   		return "", err
	   	}

	   	claims := &AuthJwtPayload{
	   		StandardClaims: jwt.StandardClaims{
	   			Subject:   user.Email,
	   			ExpiresAt: time.Now().UTC().AddDate(0, 0, config.JwtExpiryAfterDays).Unix(),
	   		},
	   	}

	   return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(authJwtKey)
	*/
}
