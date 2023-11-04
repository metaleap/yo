package yomail

import (
	"net"
	"net/smtp"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
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
	MailTo   yodb.Text
	dtDone   *yodb.DateTime
}

func CreateMailReq(ctx *Ctx, mailReq *MailReq) yodb.I64 {
	return yodb.CreateOne[MailReq](ctx, mailReq)
}

func sendMailViaSmtp(to yodb.Text, subject yodb.Text, msg string) error {
	host_addr := Cfg.YO_MAIL_SMTP_HOST + ":" + str.FromInt(Cfg.YO_MAIL_SMTP_PORT)
	mail_body := []byte(str.Repl("Subject: {subj}\r\n\r\n{body}",
		str.Dict{"subj": subject.String(), "body": str.Trim(msg)}))

	if Cfg.YO_MAIL_SMTP_TIMEOUT == 0 { // for reference/fallback really, not for actual practice
		return smtp.SendMail(
			host_addr,
			smtp.PlainAuth("", Cfg.YO_MAIL_SMTP_USERNAME, Cfg.YO_MAIL_SMTP_PASSWORD, Cfg.YO_MAIL_SMTP_HOST),
			Cfg.YO_MAIL_SMTP_SENDER, []string{to.String()},
			mail_body,
		)
	}

	conn, err := net.DialTimeout("tcp", host_addr, Cfg.YO_MAIL_SMTP_TIMEOUT)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, Cfg.YO_MAIL_SMTP_HOST)
	if client != nil {
		defer client.Quit()
	}
	if err != nil {
		return err
	}

	if err = client.Mail(Cfg.YO_MAIL_SMTP_SENDER); err != nil {
		return err
	} else if err = client.Rcpt(to.String()); err != nil {
		return err
	}

	writer, err := client.Data()
	if writer != nil {
		defer writer.Close()
	}
	if err == nil {
		_, err = writer.Write(mail_body)
	}
	return err
}
