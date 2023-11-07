package yomail

import (
	. "yo/ctx"
	yodb "yo/db"
	yojobs "yo/jobs"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

var JobTypeId = yojobs.Register[mailReqJob, Void, Void, mailReqTaskDetails, mailReqTaskResults](func(string) mailReqJob {
	return mailReqJob{}
})

var MailReqJobDef = yojobs.JobDef{
	Name:                             yodb.Text(ReflType[mailReqJob]().String()),
	JobTypeId:                        yodb.Text(JobTypeId),
	Schedules:                        yojobs.ScheduleOncePerMinute,
	TimeoutSecsTaskRun:               22,
	TimeoutSecsJobRunPrepAndFinalize: 11,
	Disabled:                         IsDevMode,
	DeleteAfterDays:                  11,
	MaxTaskRetries:                   123,
}

type mailReqTaskDetails struct{ ReqId yodb.I64 }
type mailReqTaskResults Void

type mailReqJob Void

func (me mailReqJob) JobDetails(ctx *Ctx) yojobs.JobDetails {
	return nil
}

func (mailReqJob) JobResults(_ *Ctx) (func(func() *Ctx, *yojobs.JobTask, *bool), func() yojobs.JobResults) {
	return nil, nil
}

func (mailReqJob) TaskDetails(ctx *Ctx, stream func([]yojobs.TaskDetails)) {
	reqs := yodb.FindMany[MailReq](ctx, mailReqDtDone.Equal(nil), 0, nil)
	stream(sl.To(reqs,
		func(it *MailReq) yojobs.TaskDetails { return &mailReqTaskDetails{ReqId: it.Id} }))
}

func (me mailReqJob) TaskResults(ctx *Ctx, task yojobs.TaskDetails) yojobs.TaskResults {
	task_details := task.(*mailReqTaskDetails)

	if req := yodb.FindOne[MailReq](ctx, MailReqId.Equal(task_details.ReqId)); req != nil {
		templ := Templates[string(req.TmplId)]
		if templ == nil {
			panic("no such template: '" + req.TmplId + "'")
		}
		msg := str.Repl(templ.Body, req.TmplArgs)
		err := sendMailViaSmtp(req.MailTo, yodb.Text(templ.Subject), msg)
		if err == nil {
			req.dtDone = yodb.DtNow()
			yodb.Update[MailReq](ctx, req, nil, false, MailReqFields(mailReqDtDone)...)
		} else {
			panic(err)
		}
	}
	return &mailReqTaskResults{}
}
