package yofeat_auth

import (
	"yo/db"
	"yo/util/str"
)

type UserAccount struct {
	id      db.I64
	created db.DateTime

	EmailAddr      db.Text
	PasswordHashed db.Bytes
}

func init() {
	db.Ensure[UserAccount](false, "", nil)
}

func userRegister(emailAddr string, passwordPlain string) {
	emailAddr, passwordPlain = str.Trim(emailAddr), str.Trim(passwordPlain)
	if emailAddr == "" {
		panic("ErrorEmailRequiredButMissing")
	}
	if passwordPlain == "" {
		panic("ErrorPasswordRequiredButMissing")
	}

	// exists, err := dbExists[User](me, User{Email: loginEmail})
	// if err != nil {
	// 	return err
	// } else if exists {
	// 	return ErrorSignupUserAlreadyExists
	// }
	// var hash []byte
	// if hash, err = bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.DefaultCost); err != nil {
	// 	return err
	// }
	// new_user := &User{Email: loginEmail, PasswordHashed: hash}
	// if err = dbCreate[User](me, new_user); err == nil {
	// }
}
