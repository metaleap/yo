var __yo_apiCall__timeoutMilliSec__ = 1234

var __yo_apiCall__onFailed__ = (err: any) => {
    console.error(err)
}

function _yo_apiCall__(methodPath: string, payload: any, onSuccess?: (_?: any) => void) {
    const uri = "/" + methodPath
    console.log("callAPI:", uri, payload)
    fetch(uri, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
        cache: 'no-cache', mode: 'same-origin', redirect: 'error', signal: AbortSignal.timeout(__yo_apiCall__timeoutMilliSec__)
    })
        .catch(__yo_apiCall__onFailed__)
        .then((resp: Response) => {
            if ((!resp) || (!resp.body) || (resp.status !== 200))
                return __yo_apiCall__onFailed__({ 'status_code': resp?.status, 'status_text': resp?.statusText })
            else
                resp.json()
                    .catch(__yo_apiCall__onFailed__)
                    .then((resp_json) => {
                        onSuccess(resp_json)
                    }, __yo_apiCall__onFailed__)
        }, __yo_apiCall__onFailed__)
    return false
}
