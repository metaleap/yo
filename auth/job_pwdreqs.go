package yoauth

import (
	"crypto/rand"
	"math"
	"math/big"
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	yojobs "yo/jobs"
	yomail "yo/mail"
	. "yo/util"
	sl "yo/util/sl"
	"yo/util/str"

	"golang.org/x/crypto/bcrypt"
)

const (
	MailTmplIdSignUp     = "yoauth.signUp"
	MailTmplIdPwdForgot  = "yoauth.pwdForgot"
	MailTmplVarEmailAddr = "email_addr"
	MailTmplVarName      = "name"
	MailTmplVarTmpPwd    = "pwd_tmp"
)

var AppSideTmplPopulate func(ctx *Ctx, reqTime *yodb.DateTime, emailAddr yodb.Text, existingMaybe *UserAuth, tmplArgsToPopulate yodb.JsonMap[string])

var jobTypeId = yojobs.Register[userPwdReqJob, None, None, userPwdReqTaskDetails, userPwdReqTaskResults](func(string) userPwdReqJob {
	return userPwdReqJob{}
})

var UserPwdReqJobDef = yojobs.JobDef{
	Name:                             yodb.Text(jobTypeId),
	JobTypeId:                        yodb.Text(jobTypeId),
	Schedules:                        yojobs.ScheduleOncePerMinute,
	TimeoutSecsTaskRun:               11,
	TimeoutSecsJobRunPrepAndFinalize: 11,
	Disabled:                         false,
	MaxTaskRetries:                   1, // keep low, those uncompleted `UserPwdReq`s in the DB (producing our job-tasks here) wont go away anyway
	DeleteAfterDays:                  1,
}

type userPwdReqJob None
type userPwdReqTaskDetails struct {
	ReqIdForNewMailReq yodb.I64
	ReqIdForDeletion   yodb.I64
}
type userPwdReqTaskResults struct {
	MailReqId  yodb.I64
	NumDeleted int64
}

func (me userPwdReqJob) JobDetails(ctx *Ctx) yojobs.JobDetails {
	return nil
}

func (userPwdReqJob) JobResults(_ *Ctx) (func(func() *Ctx, *yojobs.JobTask, *bool), func() yojobs.JobResults) {
	return nil, nil
}

func (userPwdReqJob) TaskDetails(ctx *Ctx, stream func([]yojobs.TaskDetails)) {
	var reqs []*UserPwdReq
	if Cfg.YO_AUTH_PWD_REQ_VALIDITY_MINS > 0 {
		dt_cutoff := time.Now().Add(-time.Minute * time.Duration(Cfg.YO_AUTH_PWD_REQ_VALIDITY_MINS+2))
		reqs = yodb.FindMany[UserPwdReq](ctx, UserPwdReqDtMade.LessThan(dt_cutoff), 0, UserPwdReqFields(UserPwdReqId))
		stream(sl.To(reqs,
			func(it *UserPwdReq) yojobs.TaskDetails { return &userPwdReqTaskDetails{ReqIdForDeletion: it.Id} }))
	}

	reqs = yodb.FindMany[UserPwdReq](ctx, UserPwdReqDoneMailReqId.Equal(nil), 0, nil) // pwd-reqs that have no corresponding mail-req yet
	stream(sl.To(reqs,
		func(it *UserPwdReq) yojobs.TaskDetails { return &userPwdReqTaskDetails{ReqIdForNewMailReq: it.Id} }))
}

func (me userPwdReqJob) TaskResults(ctx *Ctx, task yojobs.TaskDetails) yojobs.TaskResults {
	task_details, ret := task.(*userPwdReqTaskDetails), &userPwdReqTaskResults{}

	if task_details.ReqIdForDeletion > 0 {
		ret.NumDeleted = yodb.Delete[UserPwdReq](ctx, yodb.ColID.Equal(task_details.ReqIdForDeletion))
	}

	if task_details.ReqIdForNewMailReq > 0 {
		if req := yodb.FindOne[UserPwdReq](ctx, UserPwdReqId.Equal(task_details.ReqIdForNewMailReq)); req != nil {
			user := yodb.FindOne[UserAuth](ctx, UserAuthEmailAddr.Equal(req.EmailAddr))
			tmpl_id := If(user == nil, MailTmplIdSignUp, MailTmplIdPwdForgot)
			if yomail.Templates[tmpl_id] == nil {
				panic("no such mail template: '" + tmpl_id + "'")
			} else if AppSideTmplPopulate == nil {
				panic("AppSideTmplPopulate not set")
			}

			var tmp_one_time_pwd_plain string
			var tmp_one_time_pwd_hashed []byte
			for len(tmp_one_time_pwd_hashed) == 0 {
				tmp_one_time_pwd_plain = newRandomAsciiOneTimePwd(32)
				tmp_one_time_pwd_hashed, _ = bcrypt.GenerateFromPassword([]byte(tmp_one_time_pwd_plain), bcrypt.DefaultCost)
			}

			tmpl_args := yodb.JsonMap[string]{MailTmplVarEmailAddr: string(req.EmailAddr), MailTmplVarName: string(req.EmailAddr)}
			AppSideTmplPopulate(ctx, req.DtMod, req.EmailAddr, user, tmpl_args)
			tmpl_args[MailTmplVarTmpPwd] = tmp_one_time_pwd_plain

			ret.MailReqId = yomail.CreateMailReq(ctx, &yomail.MailReq{
				TmplId:   yodb.Text(tmpl_id),
				TmplArgs: tmpl_args,
				MailTo:   req.EmailAddr,
			})

			req.DoneMailReqId.SetId(ret.MailReqId)
			req.tmpPwdHashed = tmp_one_time_pwd_hashed
			yodb.Update[UserPwdReq](ctx, req, nil, false, UserPwdReqFields(UserPwdReqDoneMailReqId, userPwdReqTmpPwdHashed)...)
		}
	}

	return ret
}

func newRandomAsciiOneTimePwd(minLen int) (ret string) {
	max := big.NewInt(math.MaxInt64)
	for len(ret) < minLen {
		big, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		ret += str.FromI64(big.Int64(), 64)
	}
	return
}
