package feat_auth

import (
	"yo/db"
)

type UserAccount struct {
	id      db.Int
	created db.DateTime

	EmailAddr      db.Text
	PasswordHashed db.Bytes
}

func Use() {
	db.Ensure[UserAccount](false, nil)
}
