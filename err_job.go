package yo

import (
	"reflect"
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yodb "yo/db"
	yojobs "yo/jobs"
	yomail "yo/mail"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const mailTmplIdErrReports = "yo.errReport"

var errJobTypeId = yojobs.Register[errJob, Void, errJobResults, Void, Void](func(string) errJob { return errJob{} })

func init() {
	var dummy ErrEntry
	var mail_tmpl_body string
	ReflWalk(reflect.ValueOf(dummy), nil, true, true, func(path []any, curVal reflect.Value) {
		field_name := str.Fmt("%s", path[0])
		mail_tmpl_body += field_name + ": {" + field_name + "}\n\n"
	}, errJobReflWalkDontTraverseBut)
	yomail.Templates[mailTmplIdErrReports] = &yomail.Templ{
		Subject: "bug report: {Err} / {ErrDbRollback}",
		Body:    str.Trim(mail_tmpl_body),
	}
}

type errJob Void
type errJobResults struct{ MailReqIds []yodb.I64 }

var errJobDef = yojobs.JobDef{
	Name:                             yodb.Text(errJobTypeId),
	JobTypeId:                        yodb.Text(errJobTypeId),
	TimeoutSecsJobRunPrepAndFinalize: 44,
	DeleteAfterDays:                  2,
	RunTasklessJobs:                  true,
	Schedules:                        yojobs.ScheduleOncePerMinute, //  yojobs.ScheduleOncePerHour,
}

func (me errJob) JobDetails(_ *Ctx) yojobs.JobDetails                         { return nil }
func (errJob) TaskDetails(_ *Ctx, _ func([]yojobs.TaskDetails))               {}
func (me errJob) TaskResults(_ *Ctx, _ yojobs.TaskDetails) yojobs.TaskResults { return nil }

func (errJob) JobResults(ctx *Ctx) (func(func() *Ctx, *yojobs.JobTask, *bool), func() yojobs.JobResults) {
	return nil, func() yojobs.JobResults {
		var results errJobResults
		errs_to_report := yodb.FindMany[ErrEntry](ctx, nil, 11, nil, ErrEntryDtMod.Desc())
		var err_ids_to_delete sl.Of[yodb.I64]
		for _, err_entry := range errs_to_report {
			tmpl_args := yodb.JsonMap[string]{}
			ReflWalk(reflect.ValueOf(*err_entry), nil, true, true, func(path []any, curVal reflect.Value) {
				field_name := str.Fmt("%s", path[0])
				tmpl_args[field_name] = str.FmtV(curVal.Interface())
			}, errJobReflWalkDontTraverseBut)
			if mail_req_id := yomail.CreateMailReq(ctx, &yomail.MailReq{
				TmplId:   mailTmplIdErrReports,
				TmplArgs: tmpl_args,
				MailTo:   yodb.Text(Cfg.YO_MAIL_ERR_LOG_FWD_TO),
			}); mail_req_id > 0 {
				results.MailReqIds = sl.With(results.MailReqIds, mail_req_id)
				err_ids_to_delete = append(err_ids_to_delete, err_entry.Id)
			}
		}
		if len(err_ids_to_delete) > 0 {
			yodb.Delete[ErrEntry](ctx, ErrEntryId.In(err_ids_to_delete.ToAnys()...))
		}
		return &results
	}
}

func errJobReflWalkDontTraverseBut(fieldName string, inStruct reflect.Value) any {
	var dt **yodb.DateTime
	switch {
	case fieldName == string(ErrEntryDtMade):
		dt = ToPtr(inStruct.Interface().(ErrEntry).DtMade)
	case fieldName == string(ErrEntryDtMod):
		dt = ToPtr(inStruct.Interface().(ErrEntry).DtMod)
	}
	if dt != nil {
		if (*dt) == nil {
			return "<nil>"
		} else {
			return (*dt).Time().Format(time.RFC3339)
		}
	}
	return nil
}
