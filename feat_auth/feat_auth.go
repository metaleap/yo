package feat_auth

import (
	"yo/db"
)

type UserAccount struct {
	id      db.Int
	created db.DateTime

	EmailAddr      db.Text
	PasswordHashed db.Bytes
	FooBarBaz      db.Bool
	Data           db.Bytes
}

func Use() {
	db.Ensure[UserAccount](false, "user_acc", map[string]string{"foo": "Data"})
}
