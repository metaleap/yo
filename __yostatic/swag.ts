import van from './vanjs/van-1.2.1.debug.js'
import * as util from './util.js'

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
    let select_method: HTMLSelectElement, select_history: HTMLSelectElement, td_input: HTMLTableCellElement, td_output: HTMLTableCellElement,
        table: HTMLTableElement, input_querystring: HTMLInputElement, textarea_payload: HTMLTextAreaElement, textarea_response: HTMLTextAreaElement

    const refreshHistory = (selectLatest: boolean, selectEmpty: boolean) => {
        while (select_history.options.length > 1)
            select_history.options.remove(1)
        if (select_method.selectedOptions.length < 1)
            return
        const method_path = select_method.selectedOptions[0].value
        for (const entry of historyOf(method_path))
            select_history.options.add(html.option({ 'value': entry.dateTime }, historyEntryStr(entry)))
        if (selectEmpty || selectLatest)
            select_history.selectedIndex = (selectLatest ? 1 : 0)
    }

    const onSelectHistoryItem = () => {
        if ((select_history.selectedIndex <= 0) || (select_method.selectedIndex <= 0)) {
            input_querystring.value = ''
            return buildApiMethodGui(true)
        }
        const date_time = parseInt(select_history.selectedOptions[0].value)
        const entries = historyOf(select_method.selectedOptions[0].value)
        for (const entry of entries)
            if (entry.dateTime === date_time) {
                input_querystring.value = (entry.queryString ? JSON.stringify(entry.queryString) : '')
                textarea_payload.value = JSON.stringify(entry.payload, null, 2)
                break
            }
    }

    const buildApiTypeGui = (td: HTMLTableCellElement, isForPayload: boolean, type_name: string) => {
        const textarea = html.textarea({
            'class': 'src-json', 'readOnly': !isForPayload, 'onkeyup': () =>
                document.title = validate(apiRefl, type_name, textarea_payload.value, type_name) || ("/" + select_method.selectedOptions[0].value)
        }, '')
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

    const buildApiMethodGui = (noHistorySelect?: boolean) => {
        if (!noHistorySelect)
            refreshHistory(true, false)
        document.title = "/" + select_method.selectedOptions[0].value
        const method = apiRefl.Methods.find((_) => (_.Path === select_method.selectedOptions[0].value))
        table.style.visibility = (method ? 'visible' : 'hidden')
        buildApiTypeGui(td_input, true, method?.In)
        buildApiTypeGui(td_output, false, method?.Out)
        if (!noHistorySelect)
            onSelectHistoryItem()
    }

    const sendRequest = () => {
        const time_started = new Date().getTime()
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
            const idx = err_msg.indexOf('osition ')
            if (idx) {
                const pos_parsed = parseInt(err_msg.substring(idx + 'osition '.length))
                if (Number.isInteger(pos_parsed) && pos_parsed >= 0) {
                    textarea_payload.setSelectionRange(pos_parsed - 2, pos_parsed + 2)
                    textarea_payload.focus()
                }
            }
            return
        }
        historyStore(apiRefl, select_method.selectedOptions[0].value, payload, query_string)
        refreshHistory(true, false)
        const on_done = () => {
            const duration_ms = new Date().getTime() - time_started
            document.title = `${duration_ms}ms`
        }
        yoReq(select_method.selectedOptions[0].value, payload, (result) => {
            on_done()
            textarea_response.style.backgroundColor = '#c0f0c0'
            textarea_response.value = JSON.stringify(result, null, 2)
        }, (err, resp?: Response) => {
            on_done()
            textarea_response.style.backgroundColor = '#f0d0c0'
            textarea_response.value = JSON.stringify(err, null, 2)
            if (resp)
                resp.text().then((response_text) => textarea_response.value += ("\n" + response_text))
        }, query_string)
    }

    van.add(document.body,
        html.div({},
            select_method = html.select({ 'autofocus': true, 'onchange': (evt: UIEvent) => buildApiMethodGui() },
                ...[html.option({ 'value': '' }, '')].concat(apiRefl.Methods.map((_) => {
                    return html.option({ 'value': _.Path }, '/' + _.Path)
                }))),
            select_history = html.select({ 'style': 'max-width:80%', 'onchange': onSelectHistoryItem }, html.option({ 'value': '' }, '')),
        ),
        html.div({}, table = html.table({ 'width': '99%', 'style': 'visibility:hidden' },
            html.tr({},
                td_input = html.td({ 'width': '50%' }),
                td_output = html.td({ 'width': '50%' }),
            ),
            html.tr({}, html.td({ 'colspan': '2', 'style': 'text-align:center', 'align': 'center' },
                html.label("URL query-string obj:"),
                input_querystring = html.input({ 'type': 'text', 'value': '', 'placeholder': '{"name":"val", ...}' }),
                html.label("Login:"),
                html.input({ 'type': 'text', 'value': '', 'placeholder': 'user email addr.', 'disabled': true }),
                html.input({ 'type': 'password', 'value': '', 'placeholder': 'user password', 'disabled': true }),
                html.button({ 'style': 'font-weight:bold', 'onclick': sendRequest }, 'Go!'),
            )),
        )),
    )
    refreshHistory(false, false)
    const entry = historyLatest()
    if (entry)
        for (let i = 0; i < select_method.options.length; i++) {
            if (select_method.options[i].value === entry.methodPath) {
                select_method.selectedIndex = i
                buildApiMethodGui()
                break
            }
        }
}

function buildVal(refl: YoReflApis, type_name: string, recurse_protection: string[]): any {
    switch (type_name) {
        case 'time.Duration': return "1234ms"
        case 'time.Time': return new Date().toISOString()
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
        case '.bool': return true
        case '.string': return "foo bar"
        case '.float32': return 3.2
        case '.float64': return 6.4
        case '.int8': return -8
        case '.int16': return -16
        case '.int32': return -32
        case '.int64': return -64
        case '.uint8': return 8
        case '.uint16': return 16
        case '.uint32': return 32
        case '.uint64': return 64
    }
    return type_name
}

type HistoryEntry = {
    dateTime: number
    payload: object
    queryString?: object
}

function historyOf(methodPath: string): HistoryEntry[] {
    const json_entries = localStorage.getItem('yo:' + methodPath)
    if (json_entries) {
        const entries: HistoryEntry[] = JSON.parse(json_entries)
        return entries.reverse()
    }
    return []
}

function historyEntryStr(entry: HistoryEntry): string {
    return new Date(entry.dateTime).toLocaleString() + ": " + JSON.stringify(entry.payload) + (entry.queryString ? ("?" + JSON.stringify(entry.queryString)) : "")
}

function historyLatest() {
    let ret: undefined | (HistoryEntry & { methodPath: string }) = undefined
    for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i)
        const method_path = key.substring('yo:'.length)
        const json_entries = localStorage.getItem(key)
        const entries: HistoryEntry[] = JSON.parse(json_entries)
        for (const entry of entries)
            if ((!ret) || (entry.dateTime > ret.dateTime))
                ret = { dateTime: entry.dateTime, methodPath: method_path, payload: entry.payload, queryString: entry.queryString }
    }
    return ret
}

function historyStore(apiRefl: YoReflApis, methodPath: string, payload: object, queryString?: object) {
    const entry: HistoryEntry = {
        dateTime: new Date().getTime(),
        payload: payload,
        queryString: queryString
    }

    console.log("localStorage history house-keeping...")
    {   // since we're anyway writing to localStorage, a good moment to clean out no longer needed history entries
        const keys_to_remove: string[] = []
        for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i)
            const method_path = key.substring('yo:'.length)
            if (!apiRefl.Methods.some((_) => (_.Path === method_path))) // methodPath no longer part of API
                keys_to_remove.push(key)
            else {
                let mut = false, entries: HistoryEntry[] = JSON.parse(localStorage.getItem(key))
                for (let i = 0; i < entries.length; i++) {
                    // check for equality with current payload/queryString: anything the same can go
                    const entry = entries[i], method = apiRefl.Methods.find((_) => (_.Path === method_path))
                    const remove = ('' !== validate(apiRefl, method.In, entry.payload, method.In)) ||
                        (util.deepEq(entry.payload, payload) && util.deepEq(entry.queryString, queryString))
                    if (remove)
                        [mut, i, entries] = [true, i - 1, entries.filter((_) => (_ != entry))]
                }
                if (mut)
                    localStorage.setItem(key, JSON.stringify(entries))
            }
        }
        for (const key_to_remove of keys_to_remove) {
            console.log(`removing '${key_to_remove}' history entry due to that method no longer existing'`)
            localStorage.removeItem(key_to_remove)
        }
    }

    console.log(`storing '${methodPath}' history entry:`, entry)
    let json_entries = localStorage.getItem('yo:' + methodPath)
    if (!(json_entries && json_entries.length))
        json_entries = '[]'
    let entries: HistoryEntry[] = JSON.parse(json_entries)
    entries.push(entry)
    json_entries = JSON.stringify(entries)
    let not_stored_yet = true
    while (not_stored_yet)
        try {
            localStorage.setItem('yo:' + methodPath, json_entries)
            not_stored_yet = false
        } catch (err) {
            if (entries.length === 0) {
                console.error(err)
                break
            }
            entries = entries.slice(1)
        }
}

function validate(apiRefl: YoReflApis, type_name: string, obj: string | object, path: string, stringIsNoJson?: boolean): string {
    if ((!obj) || (obj === ''))
        return ''
    const is_str = (typeof obj === 'string'), display_path = (p: string, k?: string) => {
        return util.strTrimL(p.substring(p.indexOf('.') + 1) + (k ? ("/" + k.substring(k.indexOf(".") + 1)) : ""), "/")
    }
    for (const special_case_str_type_name of ['time.Duration', 'time.Time'])
        if (type_name === special_case_str_type_name)
            return ((is_str && obj !== '') || obj === null) ? "" : `${display_path(path)}:'${type_name}' field needs a non-empty string value or null`
    if (is_str && !stringIsNoJson)
        try {
            obj = JSON.parse(obj.toString())
        } catch (err) {
            return `${err}`
        }
    const type_struc = apiRefl.Types[type_name]
    if (obj && type_struc)
        for (const k in (obj as object)) {
            const field_type_name = type_struc[k]
            if (!field_type_name)
                return `${display_path(path, k)}: '${type_name}' has no '${k}'`
            const field_type_struc = apiRefl.Types[field_type_name]
            if (field_type_struc) {
                const err_msg = validate(apiRefl, field_type_name, (obj as object)[k], path + '/' + k, true)
                if (err_msg !== '')
                    return err_msg
            }
        }
    return ''
}
