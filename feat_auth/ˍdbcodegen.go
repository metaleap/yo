package yofeat_auth

import q "yo/db/query"

type UserAccountCol = q.C

const (
	UserAccountID             = UserAccountCol("id_")
	UserAccountCreated        = UserAccountCol("created_")
	UserAccountEmailAddr      = UserAccountCol("email_addr_")
	UserAccountPasswordHashed = UserAccountCol("password_hashed_")
)
