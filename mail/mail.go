package yomail

import (
	. "yo/ctx"
	yodb "yo/db"
)

var Templates = map[string]string{}

func init() {
	yodb.Ensure[MailReq, MailReqField]("", nil, false)
}

type MailReq struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	TmplId   yodb.Text
	TmplArgs yodb.JsonMap[string]
	MailTo   yodb.Arr[yodb.Text]
	MailCc   yodb.Arr[yodb.Text]
	MailBcc  yodb.Arr[yodb.Text]
	dtDone   *yodb.DateTime
}

func CreateMailReq(ctx *Ctx, mailReq *MailReq) yodb.I64 {
	return yodb.CreateOne[MailReq](ctx, mailReq)
}
