package yofeat_auth

import (
	. "yo/ctx"
)

func apiUserRegister(ctx *Ctx, args *struct {
	EmailAddr     string
	PasswordPlain string
}, ret *struct {
	Id int64
}) any {
	ret.Id = int64(UserRegister(ctx, args.EmailAddr, args.PasswordPlain))
	return ret
}
