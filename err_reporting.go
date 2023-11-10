package yo

import (
	"reflect"
	"time"

	. "yo/cfg"
	. "yo/ctx"
	yoctx "yo/ctx"
	yodb "yo/db"
	yojobs "yo/jobs"
	yojson "yo/json"
	yolog "yo/log"
	yomail "yo/mail"
	yosrv "yo/srv"
	. "yo/util"
	"yo/util/kv"
	"yo/util/sl"
	"yo/util/str"
)

// part 1/2: log caught panics to DB

const timeoutLogErr = 11 * time.Second

type ErrEntry struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	Err         yodb.Text
	StackTrace  yodb.Text
	CtxVals     yodb.Text
	HttpUrlPath yodb.Text
	HttpFullUri yodb.Text
	NumCaught   yodb.U8
	JobRunId    yodb.I64
	JobTaskId   yodb.I64
	DbTx        yodb.Bool
}

func init() {
	NotifyErrCaught = func(nowInvalidCtx *Ctx, ctxVals kv.Any, err any, stackTrace string) {
		if err, _ := err.(Err); (err == yosrv.ErrUnauthorized) || (err == yoctx.ErrMustBeAdmin) {
			return
		}

		ctx := NewCtxNonHttp(timeoutLogErr, false, "")
		ctx.ErrNoNotify = true
		ctx.DbNoLoggingInDevMode()
		ctx.TimingsNoPrintInDevMode = true
		defer ctx.OnDone(nil)

		var json_ctx_vals []byte
		Try(func() {
			json_ctx_vals = yojson.From(ctxVals, false)
		}, nil)

		err_entry := ErrEntry{
			Err:        yodb.Text(str.FmtV(err)),
			StackTrace: yodb.Text(stackTrace),
			NumCaught:  1,
			DbTx:       (nowInvalidCtx.Db.Tx != nil),
			CtxVals:    yodb.Text(json_ctx_vals),
		}
		if nowInvalidCtx.Job != nil {
			err_entry.JobRunId, err_entry.JobTaskId = yodb.I64(nowInvalidCtx.Job.RunId), yodb.I64(nowInvalidCtx.Job.TaskId)
		}
		if nowInvalidCtx.Http != nil {
			err_entry.HttpUrlPath = yodb.Text(nowInvalidCtx.Http.UrlPath)
			if nowInvalidCtx.Http.Req != nil {
				err_entry.HttpFullUri = yodb.Text(nowInvalidCtx.Http.Req.RequestURI)
			}
		}

		if !IsUp {
			yolog.Println(err_entry.Err.String())
			yolog.Println(err_entry.StackTrace.String())
			return
		}

		similar_enough := yodb.FindOne[ErrEntry](ctx, ErrEntryErr.Equal(err_entry.Err).
			And(ErrEntryHttpUrlPath.Equal(err_entry.HttpUrlPath)).
			And(ErrEntryJobRunId.Equal(err_entry.JobRunId)).
			And(ErrEntryDbTx.Equal(err_entry.DbTx)))
		if similar_enough == nil {
			yodb.CreateOne[ErrEntry](ctx, &err_entry)
		} else if similar_enough.NumCaught < 255 {
			similar_enough.NumCaught++
			similar_enough.CtxVals = If(len(ctxVals) == 0, similar_enough.CtxVals, err_entry.CtxVals)
			similar_enough.JobRunId = If(err_entry.JobRunId == 0, similar_enough.JobRunId, err_entry.JobRunId)
			if similar_enough.JobRunId == err_entry.JobRunId {
				similar_enough.JobTaskId = If(err_entry.JobTaskId == 0, similar_enough.JobTaskId, err_entry.JobTaskId)
			}
			similar_enough.HttpFullUri = If(err_entry.HttpFullUri == "", similar_enough.HttpFullUri, err_entry.HttpFullUri)
			similar_enough.HttpUrlPath = If(err_entry.HttpUrlPath == "", similar_enough.HttpUrlPath, err_entry.HttpUrlPath)
			similar_enough.StackTrace = If(err_entry.StackTrace == "", similar_enough.StackTrace, err_entry.StackTrace)
			similar_enough.Err = If(err_entry.Err == "", similar_enough.Err, err_entry.Err)
			yodb.Update[ErrEntry](ctx, similar_enough, nil, false)
		} // else: no-op. NumCaught==255 means "a lot, too much". dont need more, we can really stop logging any more of this...
	}
}

// job

const mailTmplIdErrReports = "yo.errReport"

var errJobTypeId = yojobs.Register[errJob, Void, errJobResults, Void, Void](func(string) errJob { return errJob{} })

func init() {
	var dummy ErrEntry
	mail_tmpl_body := "App: {App}\r\n\r\n"
	ReflWalk(reflect.ValueOf(dummy), nil, true, true, func(path []any, curVal reflect.Value) {
		field_name := str.Fmt("%s", path[0])
		mail_tmpl_body += field_name + ": {" + field_name + "}\r\n\r\n"
	}, errJobReflWalkDontTraverseBut)
	yomail.Templates[mailTmplIdErrReports] = &yomail.Templ{
		Subject: "bug report: {Err}",
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
	Schedules:                        yojobs.ScheduleOncePerHour,
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
			tmpl_args := yodb.JsonMap[string]{"App": AppPkgPath}
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
