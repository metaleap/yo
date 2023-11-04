package yomail

import (
	yodb "yo/db"
	yojobs "yo/jobs"
	. "yo/util"
	"yo/util/sl"
)

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
	DtDone   *yodb.DateTime
}

var JobTypeId = yojobs.Register[mailReqJobType, Void, Void, mailReqTaskDetails, mailReqTaskResults](func(string) mailReqJobType {
	return mailReqJobType{}
})

var MailReqJobDef = yojobs.JobDef{
	Name:                             yodb.Text(ReflType[mailReqJobType]().String()),
	JobTypeId:                        yodb.Text(JobTypeId),
	Schedules:                        yojobs.ScheduleOncePerMinute,
	TimeoutSecsTaskRun:               22,
	TimeoutSecsJobRunPrepAndFinalize: 11,
	Disabled:                         false,
	MaxTaskRetries:                   123,
}

type mailReqTaskDetails struct{ ReqId yodb.I64 }
type mailReqTaskResults struct{}

type mailReqJobType struct{}

func (me mailReqJobType) JobDetails(ctx *yojobs.Context) yojobs.JobDetails {
	return nil
}

func (mailReqJobType) JobResults(_ *yojobs.Context) (func(*yojobs.JobTask, *bool), func() yojobs.JobResults) {
	return nil, nil
}

func (mailReqJobType) TaskDetails(ctx *yojobs.Context, stream func([]yojobs.TaskDetails)) {
	reqs := yodb.FindMany[MailReq](ctx.Ctx, MailReqDtDone.Equal(nil), 0, nil)
	stream(sl.To(reqs,
		func(it *MailReq) yojobs.TaskDetails { return &mailReqTaskDetails{ReqId: it.Id} }))
}

func (me mailReqJobType) TaskResults(ctx *yojobs.Context, task yojobs.TaskDetails) yojobs.TaskResults {
	task_details := task.(*mailReqTaskDetails)

	if req := yodb.FindOne[MailReq](ctx.Ctx, MailReqId.Equal(task_details.ReqId)); req != nil {
		req.DtDone = yodb.DtNow()
		yodb.Update[MailReq](ctx.Ctx, req, nil, false, MailReqFields(MailReqDtDone)...)
	}
	return &mailReqTaskResults{}
}
