package yo

import (
	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	yojobs "yo/jobs"
	yomail "yo/mail"
	. "yo/util"
	"yo/util/sl"
)

func init() {
	yojobs.Register[errJob, Void, errJobResults, Void, Void](func(string) errJob { return errJob{} })
}

type errJob Void
type errJobResults struct{ MailReqIds []yodb.I64 }

func (me errJob) JobDetails(ctx *Ctx) yojobs.JobDetails {
	return nil
}

func (errJob) TaskDetails(ctx *Ctx, stream func([]yojobs.TaskDetails)) {
}

func (me errJob) TaskResults(ctx *Ctx, task yojobs.TaskDetails) yojobs.TaskResults {
	return nil
}

func (errJob) JobResults(ctx *Ctx) (stream func(func() *Ctx, *yojobs.JobTask, *bool), results func() yojobs.JobResults) {
	return nil, func() yojobs.JobResults {
		var results errJobResults
		errs_to_report := yodb.FindMany[ErrEntry](ctx, nil, 11, nil, ErrEntryDtMod.Desc())
		var err_ids_to_delete []yodb.I64
		for _, err_entry := range errs_to_report {
			if mail_req_id := yomail.CreateMailReq(ctx, &yomail.MailReq{
				TmplId:   "unknownTmplId",
				TmplArgs: yodb.JsonMap[string]{},
				MailTo:   yodb.Text(Cfg.YO_MAIL_ERR_LOG_FWD_TO),
			}); mail_req_id > 0 {
				results.MailReqIds = sl.With(results.MailReqIds, mail_req_id)
				err_ids_to_delete = append(err_ids_to_delete, err_entry.Id)
			}
		}
		if len(err_ids_to_delete) > 0 {
			yodb.Delete[ErrEntry](ctx, ErrEntryId.In(err_ids_to_delete))
		}
		return &results
	}
}
