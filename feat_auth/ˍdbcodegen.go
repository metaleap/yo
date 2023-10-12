package yofeat_auth

import q "yo/db/query"

type UserAccountCol = q.C

const (
	UserAccountColId             = UserAccountCol("id_")
	UserAccountColCreated        = UserAccountCol("created_")
	UserAccountColEmailAddr      = UserAccountCol("email_addr_")
	UserAccountColPasswordHashed = UserAccountCol("password_hashed_")
)

type UserAccountField q.F

const (
	UserAccountId             UserAccountField = UserAccountField("Id")
	UserAccountCreated        UserAccountField = UserAccountField("Created")
	UserAccountEmailAddr      UserAccountField = UserAccountField("EmailAddr")
	UserAccountPasswordHashed UserAccountField = UserAccountField("passwordHashed")
)
