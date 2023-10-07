import van from './vanjs/van-1.2.1.debug.js'

const html = van.tags

type YoReflType = { [_: string]: string }

type YoReflApis = {
    Methods: YoReflMethod[]
    Types: { [_: string]: YoReflType }
}

type YoReflMethod = {
    In: string
    Out: string
    Path: string
}

export function onInit(apiRefl: YoReflApis, yoReq: (methodPath: string, payload: any, onSuccess?: (_?: any) => void, onFailed?: (err: any, resp?: any) => void, query?: { [_: string]: string }) => void) {
    let select_method: HTMLSelectElement, td_input: HTMLTableCellElement, td_output: HTMLTableCellElement, table: HTMLTableElement,
        input_querystring: HTMLInputElement, textarea_payload: HTMLTextAreaElement, textarea_response: HTMLTextAreaElement

    const buildApiTypeGui = (td: HTMLTableCellElement, isForPayload: boolean, type_name: string) => {
        const textarea = html.textarea({ 'class': 'src-json', 'readOnly': !isForPayload }, '')
        if (isForPayload)
            textarea_payload = textarea
        else
            textarea_response = textarea
        td.innerHTML = ''
        if (type_name && type_name !== '') {
            const dummy_val = buildVal(apiRefl, type_name, [])
            textarea.value = JSON.stringify(dummy_val, null, 2)
            van.add(td, textarea)
        }
    }
    const buildApiMethodGui = () => {
        document.title = "/" + select_method.selectedOptions[0].value
        const method = apiRefl.Methods.find((_) => (_.Path === select_method.selectedOptions[0].value))
        table.style.visibility = (method ? 'visible' : 'hidden')
        buildApiTypeGui(td_input, true, method?.In)
        buildApiTypeGui(td_output, false, method?.Out)
    }
    const sendRequest = () => {
        // const time_started = new Date().getTime()
        textarea_response.value = "..."
        textarea_response.style.backgroundColor = '#f0f0f0'
        let query_string: { [_: string]: string }, payload: object
        if (input_querystring.value && input_querystring.value.length) {
            try { query_string = JSON.parse(input_querystring.value) } catch (err) {
                alert(`${err}`)
                return
            }
        }
        try { payload = JSON.parse(textarea_payload.value) } catch (err) {
            const err_msg = `${err}`
            alert(err_msg)
            const idx = err_msg.indexOf("osition ")
            if (idx) {
                const pos_parsed = parseInt(err_msg.substring(idx + "osition ".length) + "bla")
                if (Number.isInteger(pos_parsed) && pos_parsed >= 0) {
                    textarea_payload.setSelectionRange(pos_parsed - 2, pos_parsed + 2)
                    textarea_payload.focus()
                }
            }
            return
        }
        yoReq(select_method.selectedOptions[0].value, payload, (result) => {
            textarea_response.style.backgroundColor = '#c0f0c0'
            textarea_response.value = JSON.stringify(result, null, 2)
        }, (err, resp?: Response) => {
            textarea_response.style.backgroundColor = '#f0d0c0'
            textarea_response.value = JSON.stringify(err, null, 2)
            if (resp)
                resp.text().then((response_text) => textarea_response.value += ("\n" + response_text))
        }, query_string)
    }

    van.add(document.body,
        html.div({}, select_method = html.select({ 'autofocus': true, 'onchange': (evt: UIEvent) => buildApiMethodGui() },
            ...[html.option({ 'value': '' }, "")].concat(apiRefl.Methods.map((_) => {
                return html.option({ 'value': _.Path }, _.Path)
            })))),
        html.div({}, table = html.table({ 'width': '99%', 'style': 'visibility:hidden' },
            html.tr({},
                td_input = html.td({ 'width': '50%' }),
                td_output = html.td({ 'width': '50%' }),
            ),
            html.tr({}, html.td({ 'colspan': '2', 'style': 'text-align:center', 'align': 'center' },
                html.label("URL query-string obj:"),
                input_querystring = html.input({ 'type': 'text', 'value': '', 'placeholder': '{"name":"val", ...}' }),
                html.button({ 'style': 'font-weight:bold', 'onclick': sendRequest }, 'Go!'),
            )),
        )),
    )
}

function buildVal(refl: YoReflApis, type_name: string, recurse_protection: string[]): any {
    switch (type_name) {
        case "time.Duration": return 1234 * 1000
        case "time.Time": return new Date().toISOString()
    }
    const type_struc = refl.Types[type_name]
    if (type_struc) {
        const obj = {}
        if (recurse_protection.indexOf(type_name) >= 0)
            return null
        else
            for (const field_name in type_struc) {
                const field_type_name = type_struc[field_name]
                obj[field_name] = buildVal(refl, field_type_name, [type_name].concat(recurse_protection))
            }
        return obj
    }
    if (type_name.startsWith('[') && type_name.endsWith(']'))
        return [buildVal(refl, type_name.substring(1, type_name.length - 1), recurse_protection)]
    if (type_name.startsWith('{') && type_name.endsWith('}') && type_name.includes(':')) {
        const ret = {}, splits = type_name.substring(1, type_name.length - 1).split(':')
        ret[buildVal(refl, splits[0], recurse_protection)] = buildVal(refl, splits.slice(1).join(':'), recurse_protection)
        return ret
    }
    switch (type_name) {
        case ".bool": return true
        case ".string": return "foo bar"
        case ".float32": return 3.2
        case ".float64": return 6.4
        case ".int8": return -8
        case ".int16": return -16
        case ".int32": return -32
        case ".int64": return -64
        case ".uint8": return 8
        case ".uint16": return 16
        case ".uint32": return 32
        case ".uint64": return 64
    }
    return type_name
}
