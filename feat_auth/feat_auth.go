package feat_auth

import (
	"yo/db"
)

type UserAccount struct {
	ID             db.Int
	EmailAddr      db.Str
	PasswordHashed db.Bytes
}

func Use() {
	db.Ensure[UserAccount]()
}
