package yoauth

import (
	yodb "yo/db"
	. "yo/jobs"
	. "yo/util"
	sl "yo/util/sl"
)

var UserPwdReqJobDef = JobDef{
	Name:                             yodb.Text(ReflType[userPwdReqJobType]().String()),
	JobTypeId:                        yodb.Text(yoauthPkg.PkgName() + "." + ReflType[userPwdReqJobType]().Name()),
	Schedules:                        ScheduleOncePerMinute,
	TimeoutSecsTaskRun:               11,
	TimeoutSecsJobRunPrepAndFinalize: 11,
	MaxTaskRetries:                   11,
	DeleteAfterDays:                  11,
}

func init() {
	Register[userPwdReqJobType, Void, Void, userPwdReqTaskDetails, userPwdReqTaskResults](
		func(string) userPwdReqJobType { return userPwdReqJobType{} })
}

type userPwdReqTaskDetails struct{ ReqId yodb.I64 }
type userPwdReqTaskResults struct{ MailId yodb.I64 }

type userPwdReqJobType struct{}

func (me userPwdReqJobType) JobDetails(ctx *Context) JobDetails {
	return nil
}

func (userPwdReqJobType) JobResults(_ *Context) (func(*JobTask, *bool), func() JobResults) {
	return nil, nil
}

func (userPwdReqJobType) TaskDetails(ctx *Context, stream func([]TaskDetails)) {
	reqs := yodb.FindMany[UserPwdReq](ctx.Ctx, nil, 0, nil)
	stream(sl.To(reqs,
		func(it *UserPwdReq) TaskDetails { return &userPwdReqTaskDetails{ReqId: it.Id} }))
}

func (me userPwdReqJobType) TaskResults(ctx *Context, task TaskDetails) TaskResults {
	task_details := task.(*userPwdReqTaskDetails)

	if req := yodb.FindOne[UserPwdReq](ctx.Ctx, UserPwdReqId.Equal(task_details.ReqId)); req != nil {
		return &userPwdReqTaskResults{MailId: req.Id}
	}
	return &userPwdReqTaskResults{MailId: 0}
}
