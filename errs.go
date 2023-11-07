package yo

import (
	"time"

	yoctx "yo/ctx"
	yodb "yo/db"
	yojson "yo/json"
	. "yo/util"
	"yo/util/sl"
	"yo/util/str"
)

const timeoutLogErr = 11 * time.Second

type ErrEntry struct {
	Id     yodb.I64
	DtMade *yodb.DateTime
	DtMod  *yodb.DateTime

	Err           yodb.Text
	ErrDbRollback yodb.Text
	CtxVals       yodb.Text
	HttpUrlPath   yodb.Text
	HttpFullUri   yodb.Text
	NumCaught     yodb.U8
	JobRunId      yodb.I64
	JobTaskId     yodb.I64
}

func init() {
	yoctx.NotifyErrCaught = func(nowInvalidCtx *yoctx.Ctx, ctxVals sl.Dict, err any, errDbRollback error) {
		ctx := yoctx.NewCtxNonHttp(timeoutLogErr, false, "")
		ctx.ErrNoNotify = true
		defer ctx.OnDone(nil)
		ctx.DbTx()

		var json_ctx_vals []byte
		Try(func() {
			json_ctx_vals = yojson.From(ctxVals, false)
		}, nil)

		err_entry := ErrEntry{
			Err:           yodb.Text(str.FmtV(err)),
			ErrDbRollback: yodb.Text(str.FmtV(errDbRollback)),
			NumCaught:     1,
			CtxVals:       yodb.Text(json_ctx_vals),
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

		similar_enough := yodb.FindOne[ErrEntry](ctx,
			ErrEntryErr.Equal(err_entry.Err).And(ErrEntryErrDbRollback.Equal(err_entry.ErrDbRollback)),
		)
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
			similar_enough.Err = If(err_entry.Err == "", similar_enough.Err, err_entry.Err)
			similar_enough.ErrDbRollback = If(err_entry.ErrDbRollback == "", similar_enough.ErrDbRollback, err_entry.ErrDbRollback)
			yodb.Update[ErrEntry](ctx, similar_enough, nil, false)
		}
	}
}
