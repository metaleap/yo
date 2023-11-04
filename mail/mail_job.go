package yomail

import (
	yodb "yo/db"
	yojobs "yo/jobs"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

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
type mailReqTaskResults Void

type mailReqJobType Void

func (me mailReqJobType) JobDetails(ctx *yojobs.Context) yojobs.JobDetails {
	return nil
}

func (mailReqJobType) JobResults(_ *yojobs.Context) (func(*yojobs.JobTask, *bool), func() yojobs.JobResults) {
	return nil, nil
}

func (mailReqJobType) TaskDetails(ctx *yojobs.Context, stream func([]yojobs.TaskDetails)) {
	reqs := yodb.FindMany[MailReq](ctx.Ctx, mailReqDtDone.Equal(nil), 0, nil)
	stream(sl.To(reqs,
		func(it *MailReq) yojobs.TaskDetails { return &mailReqTaskDetails{ReqId: it.Id} }))
}

func (me mailReqJobType) TaskResults(ctx *yojobs.Context, task yojobs.TaskDetails) yojobs.TaskResults {
	task_details := task.(*mailReqTaskDetails)

	if req := yodb.FindOne[MailReq](ctx.Ctx, MailReqId.Equal(task_details.ReqId)); req != nil {
		templ := Templates[string(req.TmplId)]
		if templ == nil {
			panic("no such template: '" + req.TmplId + "'")
		}
		msg := str.Repl(templ.Body, req.TmplArgs)
		sendMailViaSmtp(req.MailTo, yodb.Text(templ.Subject), msg)
		req.dtDone = yodb.DtNow()
		yodb.Update[MailReq](ctx.Ctx, req, nil, false, MailReqFields(mailReqDtDone)...)
	}
	return &mailReqTaskResults{}
}
