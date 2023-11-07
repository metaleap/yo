package yo

import (
	"time"

	yoctx "yo/ctx"
	yodb "yo/db"
	yojson "yo/json"
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
}

func init() {
	yoctx.NotifyErrCaught = func(nowInvalidCtx *yoctx.Ctx, ctxVals sl.Dict, err any, errDbRollback error) {
		ctx := yoctx.NewCtxNonHttp(timeoutLogErr, false, "")
		ctx.ErrNoNotify = true
		defer ctx.OnDone(nil)
		ctx.DbTx()

		err_entry := ErrEntry{
			Err:           yodb.Text(str.FmtV(err)),
			ErrDbRollback: yodb.Text(str.FmtV(errDbRollback)),
			NumCaught:     1,
			CtxVals:       yodb.Text(yojson.From(ctxVals, false)),
		}
		if nowInvalidCtx.Http != nil {
			err_entry.HttpUrlPath = yodb.Text(nowInvalidCtx.Http.UrlPath)
			if nowInvalidCtx.Http.Req != nil {
				err_entry.HttpFullUri = yodb.Text(nowInvalidCtx.Http.Req.RequestURI)
			}
		}
		similar_enough := yodb.FindOne[ErrEntry](ctx,
			ErrEntryErr.Equal(err_entry.Err).
				And(ErrEntryErrDbRollback.Equal(err_entry.ErrDbRollback)).
				And(ErrEntryHttpUrlPath.Equal(err_entry.HttpUrlPath)),
		)
		if similar_enough == nil {
			yodb.CreateOne[ErrEntry](ctx, &err_entry)
		}
	}
}
