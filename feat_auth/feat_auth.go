package feat_auth

import (
	"yo/db"
)

type UserAccount struct {
	id      db.I64
	created db.DateTime

	EmailAddr      db.Text
	PasswordHashed db.Bytes
}

func Use() {
	db.Ensure[UserAccount](false, "", map[string]string{})
}
