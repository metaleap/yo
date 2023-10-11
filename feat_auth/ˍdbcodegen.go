package yofeat_auth

import q "yo/db/query"

type UserAccountCol = q.C

const (
	UserAccountID             = UserAccountCol("id")
	UserAccountCreated        = UserAccountCol("created")
	UserAccountEmailAddr      = UserAccountCol("email_addr")
	UserAccountPasswordHashed = UserAccountCol("password_hashed")
)
