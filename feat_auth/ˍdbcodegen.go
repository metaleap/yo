package yofeat_auth

import q "yo/db/query"

const (
	UserAccountID             = q.C("id")
	UserAccountCreated        = q.C("created")
	UserAccountEmailAddr      = q.C("email_addr")
	UserAccountPasswordHashed = q.C("password_hashed")
)
