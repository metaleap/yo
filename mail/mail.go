package yomail

import (
	"net/smtp"

	. "yo/ctx"
	yodb "yo/db"
	"yo/util/sl"
	"yo/util/str"
)

var Templates = map[string]*Templ{}

type Templ struct {
	Subject string
	Body    string
	Vars    []string
}

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
	dtDone   *yodb.DateTime
}

func CreateMailReq(ctx *Ctx, mailReq *MailReq) yodb.I64 {
	return yodb.CreateOne[MailReq](ctx, mailReq)
}

func send(subject yodb.Text, msg string, to ...yodb.Text) {
	err := smtp.SendMail("smtp.mailersend.net:587", smtp.PlainAuth("", "MS_v1fnUu@metaleap.net", "ccr2j0UMHGb7uQhQ", "smtp.mailersend.net"),
		"MS_v1fnUu@metaleap.net", sl.To(to, yodb.Text.String),
		[]byte(str.Repl("Subject: {subj}\r\n\r\n{body}", str.Dict{
			"subj": subject.String(), "body": str.Trim(msg),
		})))
	if err != nil {
		panic(err)
	}
}
