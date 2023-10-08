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
        table: HTMLTableElement, input_querystring: HTMLInputElement, textarea_payload: HTMLTextAreaElement, textarea_response: HTMLTextAreaElement,
        tree_payload: HTMLUListElement, tree_response: HTMLUListElement

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
        const date_time = parseInt(select_history.selectedOptions[0].value), method_path = select_method.selectedOptions[0].value
        const entries = historyOf(method_path)
        for (const entry of entries)
            if (entry.dateTime === date_time) {
                input_querystring.value = (entry.queryString ? JSON.stringify(entry.queryString) : '')
                textarea_payload.value = JSON.stringify(entry.payload, null, 2)
                refreshTree(method_path, entry.payload, tree_payload, true)
                break
            }
    }

    const buildApiTypeGui = (td: HTMLTableCellElement, isForPayload: boolean, type_name: string) => {
        const method_path = select_method.selectedOptions[0].value
        const on_textarea_changed = () => {
            const [err_msg, obj] = validate(apiRefl, type_name, textarea_payload.value, type_name)
            document.title = err_msg || ("/" + method_path)
            refreshTree(method_path, obj, isForPayload ? tree_payload : tree_response, isForPayload)
        }
        const tree = html.ul({ 'style': 'font-size:0.88em' }), textarea = html.textarea({ 'class': 'src-json', 'readOnly': !isForPayload, 'onkeyup': on_textarea_changed, 'onpaste': on_textarea_changed, 'oncut': on_textarea_changed, 'onchange': on_textarea_changed }, '')
        if (isForPayload)
            [textarea_payload, tree_payload] = [textarea, tree]
        else
            [textarea_response, tree_response] = [textarea, tree]
        td.innerHTML = ''
        if (type_name && type_name !== '') {
            const dummy_val = buildVal(apiRefl, type_name, [])
            textarea.value = JSON.stringify(dummy_val, null, 2)
            van.add(td, textarea, tree)
            refreshTree(method_path, dummy_val, isForPayload ? tree_payload : tree_response, isForPayload)
        }
    }

    const buildApiMethodGui = (noHistorySelect?: boolean) => {
        if (!noHistorySelect)
            refreshHistory(true, false)
        const method_path = select_method.selectedOptions[0].value
        document.title = "/" + method_path
        const method = apiRefl.Methods.find((_) => (_.Path === method_path))
        table.style.visibility = (method ? 'visible' : 'hidden')
        buildApiTypeGui(td_input, true, method?.In)
        buildApiTypeGui(td_output, false, method?.Out)
        if (!noHistorySelect)
            onSelectHistoryItem()
    }

    const refreshTree = (methodPath: string, obj: object, ulTree: HTMLUListElement, isForPayload: boolean) => {
        const method = apiRefl.Methods.find((_) => (_.Path === methodPath))
        const type_name = (isForPayload ? method.In : method.Out)
        refreshTreeNode(type_name, obj, ulTree, isForPayload, '')
    }

    const refreshTreeNode = (typeName: string, obj: object, ulTree: HTMLUListElement, isForPayload: boolean, path: string) => {
        const type_struc = apiRefl.Types[typeName]
        ulTree.innerHTML = ""
        if (obj && type_struc)
            for (const field_name in type_struc) {
                const field_type_name = type_struc[field_name]
                const value = obj[field_name]
                let value_elem: HTMLElement
                // 2023-10-07T19:32:10.055Z vs YYYY-MM-DDThh:mm
                if ((field_type_name === 'time.Time') && (typeof value === 'string') && (value.length >= 16) && !Number.isNaN(Date.parse(value)))
                    value_elem = html.input({ 'type': 'datetime-local', 'readOnly': !isForPayload, 'value': value.substring(0, 16) })
                else if (['.int8', '.int16', '.int32', '.int64', '.uint8', '.uint16', '.uint32', '.uint64'].some((_) => (_ === field_type_name))
                    && (typeof value === 'number'))
                    value_elem = html.input({ 'type': 'number', 'readOnly': !isForPayload, 'value': value })
                else if (['.float32', '.float64'].some((_) => (_ === field_type_name)) && (typeof value === 'number'))
                    value_elem = html.input({ 'type': 'number', 'readOnly': !isForPayload, 'step': '0.01', 'value': value })
                else if ((field_type_name === '.string') && (typeof value === 'string'))
                    value_elem = html.input({ 'type': 'text', 'readOnly': !isForPayload, 'value': value })
                else if ((field_type_name === '.bool') && (typeof value === 'boolean'))
                    value_elem = html.input({ 'type': 'checkbox', 'readOnly': !isForPayload, 'checked': value })
                else {
                    value_elem = html.ul({})
                    refreshTreeNode(field_type_name, value, value_elem as HTMLUListElement, isForPayload, path + '.' + field_name)
                    if (value_elem.innerHTML !== '')
                        value_elem.style.borderStyle = 'solid'
                }
                van.add(ulTree, html.li({ 'title': displayPath(path, field_name) }, html.span({ 'class': 'label' }, field_name + ":"), value_elem))
            }
    }

    const sendRequest = () => {
        const time_started = new Date().getTime(), show_err = (err) => {
            textarea_response.style.backgroundColor = '#f0d0c0'
            textarea_response.value = `${err}`
            refreshTree(method_path, null, tree_response, false)
        }
        textarea_response.value = "..."
        textarea_response.style.backgroundColor = '#f0f0f0'
        let query_string: { [_: string]: string }, payload: object
        if (input_querystring.value && input_querystring.value.length)
            try { query_string = JSON.parse(input_querystring.value) } catch (err) {
                return show_err(`URL query-string object:\n${err}`)
            }
        const method_path = select_method.selectedOptions[0].value
        try {
            const method = apiRefl.Methods.find((_) => (_.Path == method_path))
            const [err_msg, _] = validate(apiRefl, method.In, payload = JSON.parse(textarea_payload.value), '')
            if (err_msg && err_msg !== "")
                return show_err(err_msg)
        } catch (err) {
            const err_msg = `${err}`
            show_err(err_msg)
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
        historyStore(apiRefl, method_path, payload, query_string)
        refreshHistory(true, false)
        const on_done = () => {
            const duration_ms = new Date().getTime() - time_started
            document.title = `${duration_ms}ms`
        }
        yoReq(method_path, payload, (result) => {
            on_done()
            textarea_response.style.backgroundColor = '#c0f0c0'
            textarea_response.value = JSON.stringify(result, null, 2)
            refreshTree(method_path, result, tree_response, false)
        }, (err, resp?: Response) => {
            on_done()
            show_err(JSON.stringify(err, null, 2))
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
            select_history = html.select({ 'style': 'max-width:80%;float:right', 'onchange': onSelectHistoryItem }, html.option({ 'value': '' }, '')),
        ),
        html.div({}, table = html.table({ 'width': '99%', 'style': 'visibility:hidden' },
            html.tr({}, html.td({ 'colspan': '2', 'style': 'text-align:center', 'align': 'center' },
                html.hr(),
                html.label("URL query-string obj:"),
                input_querystring = html.input({ 'type': 'text', 'value': '', 'placeholder': '{"name":"val", ...}' }),
                html.label("Login:"),
                html.input({ 'type': 'text', 'value': '', 'placeholder': 'user email addr.', 'disabled': true }),
                html.input({ 'type': 'password', 'value': '', 'placeholder': 'user password', 'disabled': true }),
                html.button({ 'style': 'font-weight:bold', 'onclick': sendRequest }, 'Go!'),
            )),
            html.tr({},
                td_input = html.td({ 'width': '50%' }),
                td_output = html.td({ 'width': '50%' }),
            ),
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
        case 'time.Time': return new Date().toISOString()
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
                    const remove = ('' !== validate(apiRefl, method.In, entry.payload, method.In)[0]) ||
                        ((methodPath === method_path) && util.deepEq(entry.payload, payload) && util.deepEq(entry.queryString, queryString))
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

function displayPath(p: string, k?: string) {
    return p + (k ? ("." + k) : "")
}

function validate(apiRefl: YoReflApis, type_name: string, value: any, path: string, stringIsNoJson?: boolean): [string, any] {
    const is_str = (typeof value === 'string')
    if (value === undefined)
        return [`${displayPath(path)}: new bug, 'value' being 'undefined'`, undefined]

    if (type_name === 'time.Time') {
        if (!((is_str && value !== '') || (value === null)))
            return [`${displayPath(path)}: must be non-empty string or null`, undefined]
        else if (is_str && value && Number.isNaN(Date.parse(value.toString())))
            return [`${displayPath(path)}: must be 'Date.parse'able`, undefined]
        else
            return ["", value]
    }

    if (type_name.startsWith('.') && (value !== null)) {
        if (['.float32', '.float64'].some((_) => (_ === type_name)) && (typeof value !== 'number'))
            return [`${displayPath(path)}: must be float, not ${JSON.stringify(value)}`, undefined]
        if (('.bool' === type_name) && (typeof value !== 'boolean'))
            return [`${displayPath(path)}: must be true or false, not ${JSON.stringify(value)}`, undefined]
        if (('.string' === type_name) && (typeof value !== 'string'))
            return [`${displayPath(path)}: must be string, not ${JSON.stringify(value)}`, undefined]
        const value_i = ((typeof value === 'number') && (value.toString().includes('.') || value.toString().includes('e')))
            ? Number.NaN : parseInt(value)
        if (['.uint8', '.uint16', '.uint32', '.uint64', '.int8', '.int16', '.int32', '.int64'].some((_) => (_ === type_name)) && ((typeof value !== 'number') || Number.isNaN(value_i)))
            return [`${displayPath(path)}: must be integer, not ${JSON.stringify(value)}`, undefined]
        if (['.uint8', '.uint16', '.uint32', '.uint64'].some((_) => (_ === type_name)) && (value_i < 0))
            return [`${displayPath(path)}: must be greater than 0, not ${JSON.stringify(value)}`, undefined]
        return ["", value]
    }

    if (is_str && value && !stringIsNoJson)
        try {
            value = JSON.parse(value.toString())
        } catch (err) {
            return [`${err}`, undefined]
        }

    if (type_name.startsWith('[') && type_name.endsWith(']') && value) {
        if (!Array.isArray(value))
            return [`${displayPath(path)}: must be null or ${type_name}, not ${value}`, undefined]
        for (const i in (value as [])) {
            const item = (value as [])[i]
            const [err_msg, _] = validate(apiRefl, type_name.substring(1, type_name.length - 1), item, path + '[' + i + ']', true)
            if (err_msg && err_msg !== "")
                return [err_msg, undefined]
        }
    }

    if (value && (typeof value !== 'object'))
        return [`${displayPath(path)}: must be null or ${type_name}, not ${value}`, undefined]

    if (type_name.startsWith('{') && type_name.endsWith('}') && value) {
        const splits = type_name.substring(1, type_name.length - 1).split(':')
        for (const key in (value as object)) {
            const [err_msg_key, _] = validate(apiRefl, splits[0], key, path + '["' + key + '"]', true)
            if (err_msg_key && err_msg_key !== "")
                return [err_msg_key, undefined]
            const val = value[key]
            const [err_msg_val, __] = validate(apiRefl, splits.slice(1).join(':'), val, path + '["' + key + '"]', true)
            if (err_msg_val && err_msg_val !== "")
                return [err_msg_val, undefined]
        }
    }

    const type_struc = apiRefl.Types[type_name]
    if (type_struc && value) {
        const type_struc_field_names = []
        for (const type_field_name in type_struc)
            type_struc_field_names.push(type_field_name)
        for (const k in (value as object)) {
            const field_type_name = type_struc[k]
            if (!field_type_name)
                return [`${displayPath(path, k)}: '${type_name}' has no '${k}' but has: '${type_struc_field_names.join("', '")}'`, undefined]
            const [err_msg, _] = validate(apiRefl, field_type_name, (value as object)[k], path + '.' + k, true)
            if (err_msg !== '')
                return [err_msg, undefined]
        }
    }
    return ["", value]
}
