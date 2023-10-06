type Yo_i8 = number
type Yo_i16 = number
type Yo_i32 = number
type Yo_i64 = number
type Yo_u8 = number
type Yo_u16 = number
type Yo_u32 = number
type Yo_u64 = number
type Yo_f32 = number
type Yo_f64 = number

var yoReq_timeoutMilliSec = 1234

var yoReq_OnFailed = (err: any) => {
    console.error(err)
}

function yoReq(methodPath: string, payload: any, onSuccess?: (_?: any) => void) {
    const uri = "/" + methodPath
    console.log("callAPI:", uri, payload)
    fetch(uri, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
        cache: 'no-cache', mode: 'same-origin', redirect: 'error', signal: AbortSignal.timeout(yoReq_timeoutMilliSec)
    })
        .catch(yoReq_OnFailed)
        .then((resp: Response) => {
            if ((!resp) || (!resp.body) || (resp.status !== 200))
                return yoReq_OnFailed({ 'status_code': resp?.status, 'status_text': resp?.statusText })
            else
                resp.json()
                    .catch(yoReq_OnFailed)
                    .then((resp_json) => {
                        onSuccess(resp_json)
                    }, yoReq_OnFailed)
        }, yoReq_OnFailed)
    return false
}
