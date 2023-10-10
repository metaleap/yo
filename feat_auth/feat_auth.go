package feat_auth

import (
	"yo/db"
)

type UserAccount struct {
	db.Base
	EmailAddr      db.Str
	PasswordHashed db.Bytes
}

func Use() {
	db.Ensure[UserAccount](false)
}
