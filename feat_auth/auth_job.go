package yoauth

import (
	yodb "yo/db"
	yojobs "yo/jobs"
	yomail "yo/mail"
	. "yo/util"
	sl "yo/util/sl"
)

const ( // change those only together with the tmpls in `init`
	MailTmplIdSignUp     = "yoauth.signUp"
	MailTmplIdPwdForgot  = "yoauth.pwdForgot"
	MailTmplVarEmailAddr = "email_addr"
	MailTmplVarName      = "name"
	MailTmplVarTmpPwd    = "pwd_tmp"
	MailTmplVarHref      = "href"
)

type UserPwdReq struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	EmailAddr yodb.Text
	DoneId    yodb.I64
}

var AppSideTmplPopulate func(yodb.JsonMap[string])

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
	MaxTaskRetries:                   11,
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
	reqs := yodb.FindMany[UserPwdReq](ctx.Ctx, UserPwdReqDoneId.Equal(0), 0, nil)
	stream(sl.To(reqs,
		func(it *UserPwdReq) yojobs.TaskDetails { return &userPwdReqTaskDetails{ReqId: it.Id} }))
}

func (me userPwdReqJobType) TaskResults(ctx *yojobs.Context, task yojobs.TaskDetails) yojobs.TaskResults {
	task_details, ret := task.(*userPwdReqTaskDetails), &userPwdReqTaskResults{}

	if req := yodb.FindOne[UserPwdReq](ctx.Ctx, UserPwdReqId.Equal(task_details.ReqId)); req != nil {
		user := yodb.FindOne[UserAuth](ctx.Ctx, UserAuthEmailAddr.Equal(req.EmailAddr))
		tmpl_id := If(user == nil, MailTmplIdSignUp, MailTmplIdPwdForgot)
		tmpl := yomail.Templates[tmpl_id]
		if tmpl == "" {
			panic("no such mail template: '" + tmpl_id + "'")
		} else if AppSideTmplPopulate == nil {
			panic("AppSideTmplPopulate not set")
		}

		tmpl_args := yodb.JsonMap[string]{}
		AppSideTmplPopulate(tmpl_args)
		tmpl_args[MailTmplVarEmailAddr] = string(req.EmailAddr)
		ret.MailReqId = yomail.CreateMailReq(ctx.Ctx, &yomail.MailReq{
			TmplId:   yodb.Text(tmpl_id),
			TmplArgs: tmpl_args,
			MailTo:   yodb.Arr[yodb.Text]{req.EmailAddr},
		})

		req.DoneId = ret.MailReqId
		yodb.Update[UserPwdReq](ctx.Ctx, req, nil, false, UserPwdReqFields(UserPwdReqDoneId)...)
	}
	return ret
}
