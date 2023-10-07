export type Yo_i8 = number
export type Yo_i16 = number
export type Yo_i32 = number
export type Yo_i64 = number
export type Yo_u8 = number
export type Yo_u16 = number
export type Yo_u32 = number
export type Yo_u64 = number
export type Yo_f32 = number
export type Yo_f64 = number

export let yoReq_timeoutMilliSec = 1234

let yoReq_OnFailed = (err: any, resp?: Response) => {
    console.error(err, resp)
}

export function setReqTimeoutMilliSec(timeout: number) {
    yoReq_timeoutMilliSec = timeout
}

export function setOnFailed(onFailed: (err: any, resp?: Response) => void) {
    yoReq_OnFailed = onFailed
}

export function yoReq(methodPath: string, payload: any, onSuccess?: (_?: any) => void, onFailed?: (err: any, resp?: Response) => void, query?: { [_: string]: string }) {
    let uri = "/" + methodPath
    if (query)
        uri += '?' + new URLSearchParams(query).toString()
    console.log("callAPI:", uri, payload)
    if (!onFailed)
        onFailed = yoReq_OnFailed
    fetch(uri, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
        cache: 'no-cache', mode: 'same-origin', redirect: 'error', signal: AbortSignal.timeout(yoReq_timeoutMilliSec)
    })
        .catch(onFailed)
        .then((resp: Response) => {
            if ((!resp) || (!resp.body) || (resp.status !== 200))
                return onFailed({ 'status_code': resp?.status, 'status_text': resp?.statusText }, resp)
            else
                resp.json()
                    .catch((err) => onFailed(err, resp))
                    .then((resp_json) => {
                        if (onSuccess)
                            onSuccess(resp_json)
                    }, (err) => onFailed(err, resp))
        }, onFailed)
    return false
}
