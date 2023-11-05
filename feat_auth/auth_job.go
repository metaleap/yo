package yoauth

import (
	"math"
	"math/rand"

	yodb "yo/db"
	yojobs "yo/jobs"
	yomail "yo/mail"
	. "yo/util"
	sl "yo/util/sl"
	"yo/util/str"

	"golang.org/x/crypto/bcrypt"
)

const ( // change those only together with the tmpls in `init`
	MailTmplIdSignUp     = "yoauth.signUp"
	MailTmplIdPwdForgot  = "yoauth.pwdForgot"
	MailTmplVarEmailAddr = "email_addr"
	MailTmplVarName      = "name"
	MailTmplVarTmpPwd    = "pwd_tmp"
)

var AppSideTmplPopulate func(ctx *yojobs.Context, reqTime *yodb.DateTime, emailAddr yodb.Text, existingMaybe *UserAuth, tmplArgsToPopulate yodb.JsonMap[string])

var JobTypeId = yojobs.Register[userPwdReqJobType, Void, Void, userPwdReqTaskDetails, userPwdReqTaskResults](func(string) userPwdReqJobType {
	return userPwdReqJobType{}
})

var UserPwdReqJobDef = yojobs.JobDef{
	Name:                             yodb.Text(ReflType[userPwdReqJobType]().String()),
	JobTypeId:                        yodb.Text(JobTypeId),
	Schedules:                        yojobs.ScheduleOncePerMinute,
	TimeoutSecsTaskRun:               11,
	TimeoutSecsJobRunPrepAndFinalize: 11,
	Disabled:                         false,
	MaxTaskRetries:                   1, // keep low, those uncompleted `UserPwdReq`s in the DB (producing our job-tasks here) wont go away anyway
	DeleteAfterDays:                  11,
}

type userPwdReqTaskDetails struct{ ReqId yodb.I64 }
type userPwdReqTaskResults struct{ MailReqId yodb.I64 }
type userPwdReqJobType Void

func (me userPwdReqJobType) JobDetails(ctx *yojobs.Context) yojobs.JobDetails {
	return nil
}

func (userPwdReqJobType) JobResults(_ *yojobs.Context) (func(*yojobs.JobTask, *bool), func() yojobs.JobResults) {
	return nil, nil
}

func (userPwdReqJobType) TaskDetails(ctx *yojobs.Context, stream func([]yojobs.TaskDetails)) {
	reqs := yodb.FindMany[UserPwdReq](ctx.Ctx, UserPwdReqDoneMailReqId.Equal(nil), 0, nil)
	stream(sl.To(reqs,
		func(it *UserPwdReq) yojobs.TaskDetails { return &userPwdReqTaskDetails{ReqId: it.Id} }))
}

func (me userPwdReqJobType) TaskResults(ctx *yojobs.Context, task yojobs.TaskDetails) yojobs.TaskResults {
	task_details, ret := task.(*userPwdReqTaskDetails), &userPwdReqTaskResults{}

	if req := yodb.FindOne[UserPwdReq](ctx.Ctx, UserPwdReqId.Equal(task_details.ReqId)); req != nil {
		user := yodb.FindOne[UserAuth](ctx.Ctx, UserAuthEmailAddr.Equal(req.EmailAddr))
		tmpl_id := If(user == nil, MailTmplIdSignUp, MailTmplIdPwdForgot)
		if yomail.Templates[tmpl_id] == nil {
			panic("no such mail template: '" + tmpl_id + "'")
		} else if AppSideTmplPopulate == nil {
			panic("AppSideTmplPopulate not set")
		}

		var tmp_one_time_pwd_plain string
		var tmp_one_time_pwd_hashed []byte
		for len(tmp_one_time_pwd_hashed) == 0 {
			tmp_one_time_pwd_plain = newRandomAsciiOneTimePwd(22)
			tmp_one_time_pwd_hashed, _ = bcrypt.GenerateFromPassword([]byte(tmp_one_time_pwd_plain), bcrypt.DefaultCost)
		}

		tmpl_args := yodb.JsonMap[string]{MailTmplVarEmailAddr: string(req.EmailAddr), MailTmplVarName: string(req.EmailAddr)}
		AppSideTmplPopulate(ctx, req.DtMod, req.EmailAddr, user, tmpl_args)
		tmpl_args[MailTmplVarTmpPwd] = tmp_one_time_pwd_plain

		ret.MailReqId = yomail.CreateMailReq(ctx.Ctx, &yomail.MailReq{
			TmplId:   yodb.Text(tmpl_id),
			TmplArgs: tmpl_args,
			MailTo:   req.EmailAddr,
		})

		req.DoneMailReqId.SetId(ret.MailReqId)
		req.tmpPwdHashed = tmp_one_time_pwd_hashed
		yodb.Update[UserPwdReq](ctx.Ctx, req, nil, false, UserPwdReqFields(UserPwdReqDoneMailReqId, userPwdReqTmpPwdHashed)...)
	}
	return ret
}

func newRandomAsciiOneTimePwd(minLen int) (ret string) {
	for len(ret) < minLen {
		ret += str.FromI64(rand.Int63n(math.MaxInt64), 36)
	}
	return
}
