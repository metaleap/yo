package yoauth

import (
	yodb "yo/db"
	yojobs "yo/jobs"
	. "yo/util"
	sl "yo/util/sl"
)

type UserPwdReq struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	EmailAddr yodb.Text
	DoneId    yodb.I64
}

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
type userPwdReqTaskResults struct{ MailId yodb.I64 }

type userPwdReqJobType struct{}

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
	task_details := task.(*userPwdReqTaskDetails)

	if req := yodb.FindOne[UserPwdReq](ctx.Ctx, UserPwdReqId.Equal(task_details.ReqId)); req != nil {
		req.DoneId = req.Id
		yodb.Update[UserPwdReq](ctx.Ctx, req, nil, false, UserPwdReqFields(UserPwdReqDoneId)...)
		return &userPwdReqTaskResults{MailId: req.Id}
	}
	return &userPwdReqTaskResults{MailId: 0}
}
