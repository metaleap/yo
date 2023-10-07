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

export function onInit(apiRefl: YoReflApis) {
    let select_method: HTMLSelectElement, td_input: HTMLTableCellElement, td_output: HTMLTableCellElement,
        table: HTMLTableElement, input_querystring: HTMLInputElement, textarea_payload: HTMLTextAreaElement

    const buildApiTypeGui = (td: HTMLTableCellElement, readOnly: boolean, type_name: string) => {
        const textarea = html.textarea({ 'class': 'src-json', 'readOnly': readOnly }, '')
        if (!readOnly)
            textarea_payload = textarea
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
        buildApiTypeGui(td_input, false, method?.In)
        buildApiTypeGui(td_output, true, method?.Out)
    }
    const sendRequest = () => {
        let query_string: { [_: string]: string }, payload: object
        if (input_querystring.value && input_querystring.value.length) {
            try { query_string = JSON.parse(input_querystring.value) } catch (err) {
                alert(`Not valid JSON: '${input_querystring.value}'\n\n(${err})`)
                return
            }
        }
        try { payload = JSON.parse(textarea_payload.value) } catch (err) {
            alert(`Not valid JSON: '${textarea_payload.value}'\n\n(${err})`)
            return
        }
        yoReq(select_method.selectedOptions[0].value, payload, () => { }, query_string)
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
            html.tr({}, html.td({ 'colspan': '2' },
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
