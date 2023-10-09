import van from './vanjs/van-1.2.1.debug.js'
import * as util from './util.js'

const undef = void 0
const html = van.tags

type YoReflType = { [_: string]: string }

type YoReflApis = {
    Methods: YoReflMethod[]
    Types: { [_: string]: YoReflType }
    Enums: { [_: string]: string[] }
}

type YoReflMethod = {
    In: string
    Out: string
    Path: string
}

export function onInit(parent: HTMLElement, apiRefl: YoReflApis, yoReq: (methodPath: string, payload: any, onSuccess?: (_?: any) => void, onFailed?: (err: any, resp?: any) => void, query?: { [_: string]: string }) => void) {
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
            select_history.options.add(html.option({ 'value': entry.dateTime }, historyEntryStr(entry, 123)))
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
        let last_textarea_value = ''
        const on_textarea_maybe_modified = () => {
            if ((!isForPayload) || (textarea_payload.value === last_textarea_value)) // not every keyup is a modify
                return
            last_textarea_value = textarea_payload.value
            const [err_msg, obj] = validate(apiRefl, type_name, textarea_payload.value, type_name)
            document.title = err_msg || ("/" + method_path)
            refreshTree(method_path, obj, isForPayload ? tree_payload : tree_response, isForPayload)
            textarea_response.style.backgroundColor = '#f0f0f0'
        }
        const tree = html.ul({ 'style': 'font-size:0.88em' }), textarea = html.textarea({ 'class': 'src-json', 'readOnly': !isForPayload, 'onkeyup': on_textarea_maybe_modified, 'onpaste': on_textarea_maybe_modified, 'oncut': on_textarea_maybe_modified, 'onchange': on_textarea_maybe_modified }, '')
        if (isForPayload)
            [textarea_payload, tree_payload] = [textarea, tree]
        else
            [textarea_response, tree_response] = [textarea, tree]
        td.innerHTML = ''
        if (type_name && type_name !== '') {
            const dummy_val = newSampleVal(apiRefl, type_name, [])
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

    const selJsonFromTree = (path: string, isForPayload: boolean, noFocus?: boolean) => {
        const textarea = (isForPayload ? textarea_payload : textarea_response)
        let level = 1, value = JSON.parse(textarea.value)
        textarea.value = JSON.stringify(value, null, 2)
        const path_parts = util.strTrimL(path, '.').split('.')
        let prev_pos = 0, prev_json = textarea.value, text_sel_pos = -1, text_sel_len = 0
        for (const path_part of path_parts) {
            let key = path_part
            if (path_part.startsWith('["') && path_part.endsWith('"]')) {
                key = path_part.substring(2, path_part.length - 2)
                value = value[key]
            } else if (path_part.startsWith('[') && path_part.endsWith(']')) {
                key = ''
                value = value[parseInt(path_part.substring(1))]
            } else
                value = value[path_part]

            let json_val = JSON.stringify(value, null, 2)
            if (json_val === undef) // fresh removal via tree editor
                break
            for (let i = 0; i < json_val.length; i++)
                if (json_val.charAt(i) === '\n')
                    json_val = json_val.substring(0, i + 1) + '  '.repeat(level) + json_val.substring(i + 1)
            const needle_prefix = `\n${'  '.repeat(level)}` + ((key === '') ? ('') : (`"${key}": `))
            const needle = needle_prefix + json_val
            let pos_cur_first = prev_json.indexOf(needle), pos_cur_last = prev_json.lastIndexOf(needle)
            if ((pos_cur_first < 0) || (pos_cur_first !== pos_cur_last))
                break
            const pos = prev_pos + (pos_cur_first + needle_prefix.length)
            level++
            [text_sel_pos, text_sel_len, prev_pos, prev_json] = [pos, json_val.length, pos, json_val]
        }
        if ((text_sel_pos >= 0) && (text_sel_len > 0)) {
            const text_sel_start = text_sel_pos, text_sel_end = text_sel_pos + text_sel_len
            { // ensuring select+scrollTo as per https://stackoverflow.com/a/53082182
                if (!noFocus)
                    textarea.focus()
                const full_text = textarea.value
                textarea.value = full_text.substring(0, text_sel_start /* quoted post uses end, but we wanna rather see the start of the selection than its end for big selections */)
                textarea.scrollTop = textarea.scrollHeight
                textarea.value = full_text
                textarea.setSelectionRange(text_sel_start, text_sel_end, "backward")
            }
        }
        return false
    }

    const refreshTree = (methodPath: string, obj: object, ulTree: HTMLUListElement, isForPayload: boolean) => {
        const method = apiRefl.Methods.find((_) => (_.Path === methodPath))
        const type_name = (isForPayload ? method.In : method.Out)
        refreshTreeNode(type_name, obj, ulTree, isForPayload, '', obj)
    }

    const refreshTreeNode = (typeName: string, value: any, ulTree: HTMLUListElement, isForPayload: boolean, path: string, root: any) => {
        const type_struc = apiRefl.Types[typeName], is_array = Array.isArray(value)
        ulTree.innerHTML = ""
        if (!value)
            return
        const buildItemInput = (itemTypeName: string, key: string, val: any) => {
            let field_input: HTMLElement, checkbox: HTMLInputElement, get_val: (_: string) => any
            const on_change = (evt: UIEvent) => {
                let index: string | number = key, refresh_tree = false
                if (key.startsWith('["') && key.endsWith('"]'))
                    index = key.substring(2, key.length - 2)
                else if (key.startsWith('[') && key.endsWith(']') && is_array)
                    index = parseInt(key.substring(1))
                if (!get_val) {
                    const sub_val = fieldInputValue(value[index], is_array)
                    if ((checkbox.checked) && ((sub_val === null) || (sub_val === undef))) {
                        const new_val = fieldInputValue(newSampleVal(apiRefl, itemTypeName, []), is_array)
                        if (new_val === undef)
                            checkbox.checked = false
                        else
                            [refresh_tree, value[index]] = [true, new_val]
                    }
                } else if (checkbox.checked) {
                    const v = fieldInputValue(get_val((field_input as any).value),
                        is_array || ((typeof val === 'number') && (evt.currentTarget === field_input)))
                    if (v !== undef)
                        value[index] = v
                    else if (is_array)
                        value[index] = null
                    else {
                        const is_new_val_check = (evt.currentTarget === checkbox)
                        const new_val = (((itemTypeName === '.bool') || (itemTypeName === '.string')) && !is_new_val_check)
                            ? undef : fieldInputValue(newSampleVal(apiRefl, itemTypeName, []), is_array)
                        if (new_val === undef)
                            checkbox.checked = false
                        else
                            [refresh_tree, value[index]] = [true, new_val]
                    }
                }
                if (field_input)
                    if (get_val) // field input control
                        field_input.style.visibility = (checkbox.checked ? 'visible' : 'hidden')
                    else // sub-tree
                        field_input.style.display = (checkbox.checked ? 'block' : 'none')
                if (!checkbox.checked)
                    if (typeof index === 'number')
                        value[index] = null
                    else
                        delete value[index]
                const textarea = (isForPayload ? textarea_payload : textarea_response)
                textarea.value = JSON.stringify(root, null, 2)
                textarea_response.style.backgroundColor = '#f0f0f0'
                selJsonFromTree(path + '.' + key, isForPayload, !refresh_tree)
                if (refresh_tree)
                    refreshTreeNode(typeName, value, ulTree, isForPayload, path, root)
            }
            if ((itemTypeName === 'time.Time') && (typeof val === 'string') && (val.length >= 16) && !Number.isNaN(Date.parse(val))) {
                field_input = html.input({ 'onchange': on_change, 'type': 'datetime-local', 'readOnly': !isForPayload, 'value': val.substring(0, 16) /* must be YYYY-MM-DDThh:mm */ })
                get_val = (s: string) => new Date(Date.parse(s)).toISOString()
            } else if (['.int8', '.int16', '.int32', '.int64', '.uint8', '.uint16', '.uint32', '.uint64'].some((_) => (_ === itemTypeName))
                && (typeof val === 'number')) {
                field_input = html.input({ 'onchange': on_change, 'type': 'number', 'readOnly': !isForPayload, 'value': val, 'min': numTypeMin(itemTypeName), 'max': numTypeMax(itemTypeName) })
                get_val = (s: string) => parseInt(s)
            } else if (['.float32', '.float64'].some((_) => (_ === itemTypeName)) && (typeof val === 'number')) {
                field_input = html.input({ 'onchange': on_change, 'type': 'number', 'readOnly': !isForPayload, 'step': '0.01', 'value': val, 'min': numTypeMin(itemTypeName), 'max': numTypeMax(itemTypeName) })
                get_val = (s: string) => parseFloat(s)
            } else if ((itemTypeName === '.string') && (typeof val === 'string')) {
                field_input = html.input({ 'onchange': on_change, 'type': 'text', 'readOnly': !isForPayload, 'value': val })
                get_val = (s: string) => s
            } else if ((itemTypeName === '.bool') && (typeof val === 'boolean')) {
                field_input = html.input({ 'onchange': on_change, 'type': 'checkbox', 'disabled': !isForPayload, 'checked': val })
                get_val = (_: string) => (field_input as HTMLInputElement).checked
            } else if (enumExists(apiRefl, itemTypeName) && (typeof val === 'string')) {
                const enumerants = apiRefl.Enums[itemTypeName]
                if (enumerants && (enumerants.length > 0) && (enumerants.indexOf(val) >= 0))
                    field_input = html.select({ 'onchange': on_change }, ...enumerants.map((_) => html.option({ 'value': _, 'selected': (_ === val) }, _)))
                else
                    field_input = html.input({ 'onchange': on_change, 'type': 'text', 'readOnly': !isForPayload, 'value': val })
                get_val = (s: string) => s
            }
            let field_input_subs = false, field_input_got = (field_input ? true : false)
            if (field_input_subs = (!field_input_got)) {
                field_input = html.ul({})
                refreshTreeNode(itemTypeName, val, field_input as HTMLUListElement, isForPayload, path + '.' + key, root)
                if (field_input_got = (field_input.innerHTML !== ''))
                    field_input.style.borderStyle = 'solid'
            }
            van.add(ulTree, html.li({ 'title': displayPath(path, key) },
                checkbox = html.input({ 'onchange': on_change, 'type': 'checkbox', 'disabled': !isForPayload, 'checked': field_input_got }),
                html.a({ 'class': 'label', 'style': (field_input_subs ? 'width:auto' : ''), 'onclick': () => selJsonFromTree(path + '.' + key, isForPayload) },
                    (key.startsWith('[') ? "" : ".") + key),
                field_input,
            ))
        }

        const is_arr = (typeName.startsWith('[') && typeName.endsWith(']')), is_map = (typeName.startsWith('{') && typeName.endsWith('}') && typeName.includes(':'))
        if (is_arr || is_map) {
            if ((is_arr && !is_array) || (is_map && (typeof value !== 'object')))
                return
            for (const key in value) {
                const val = value[key]
                let type_name = typeName.substring(1, typeName.length - 1)
                if (is_map)
                    type_name = type_name.substring(type_name.indexOf(':') + 1)
                buildItemInput(type_name, (is_arr ? `[${key}]` : `["${key}"]`), val)
            }
        } else if (type_struc)
            for (const field_name in type_struc) {
                const field_type_name = type_struc[field_name], field_val = value[field_name]
                buildItemInput(field_type_name, field_name, field_val)
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

    const openInNewDialog = () => {
        const dialog = html.dialog({ 'style': 'width:88%' })
        van.add(parent, dialog)
        dialog.onclose = () => { dialog.remove() }
        onInit(dialog, apiRefl, yoReq)
        dialog.showModal()
    }

    van.add(parent,
        html.div({},
            select_method = html.select({ 'autofocus': true, 'onchange': (evt: UIEvent) => buildApiMethodGui() },
                ...[html.option({ 'value': '' }, '')].concat(apiRefl.Methods.map((_) => {
                    return html.option({ 'value': _.Path }, _.Path)
                }))),
            html.button({ 'style': 'margin-left:1em', 'onclick': openInNewDialog }, 'New Dialog...'),
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

function newSampleVal(refl: YoReflApis, type_name: string, recurse_protection: string[]): any {
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

    if (enumExists(refl, type_name)) {
        const enumerants = refl.Enums[type_name]
        if (enumerants && (enumerants.length > 0))
            return enumerants.join('|')
        return `(some ${type_name} enumerant)`
    }

    const type_struc = refl.Types[type_name]
    if (type_struc) {
        const obj = {}
        if (recurse_protection.indexOf(type_name) >= 0)
            return null
        else
            for (const field_name in type_struc) {
                const field_type_name = type_struc[field_name]
                obj[field_name] = newSampleVal(refl, field_type_name, [type_name].concat(recurse_protection))
            }
        return obj
    }
    if (type_name.startsWith('[') && type_name.endsWith(']'))
        return [newSampleVal(refl, type_name.substring(1, type_name.length - 1), recurse_protection)]
    if (type_name.startsWith('{') && type_name.endsWith('}') && type_name.includes(':')) {
        const ret = {}, splits = type_name.substring(1, type_name.length - 1).split(':')
        ret[newSampleVal(refl, splits[0], recurse_protection)] = newSampleVal(refl, splits.slice(1).join(':'), recurse_protection)
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

function historyEntryStr(entry: HistoryEntry, maxLen: number = 0): string {
    const ret = new Date(entry.dateTime).toLocaleString() + ": " + JSON.stringify(entry.payload) + (entry.queryString ? ("?" + JSON.stringify(entry.queryString)) : "")
    return ((maxLen > 0) && (ret.length > maxLen)) ? (ret.substring(0, maxLen) + '...') : ret
}

function historyLatest() {
    let ret: undefined | (HistoryEntry & { methodPath: string }) = undef
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

function enumExists(apiRefl: YoReflApis, type_name: string) {
    return (Object.keys(apiRefl.Enums).indexOf(type_name) >= 0)
}

function validate(apiRefl: YoReflApis, type_name: string, value: any, path: string, stringIsNoJson?: boolean): [string, any] {
    const is_str = (typeof value === 'string')
    if (value === undef)
        return [`${displayPath(path)}: new bug, 'value' being 'undefined'`, undef]

    if (type_name === 'time.Time') {
        if (!((is_str && value !== '') || (value === null)))
            return [`${displayPath(path)}: must be non-empty string or null`, undef]
        else if (is_str && value && Number.isNaN(Date.parse(value.toString())))
            return [`${displayPath(path)}: must be 'Date.parse'able`, undef]
        return ["", value]
    }

    if (enumExists(apiRefl, type_name)) {
        if (!((is_str && value !== '') || (value === null)))
            return [`${displayPath(path)}: must be must be non-empty string or null`, undef]
        const enumerants = apiRefl.Enums[type_name]
        if (enumerants && (enumerants.length > 0) && (enumerants.indexOf(value) < 0))
            return [`${displayPath(path)}: '${type_name}' has no '${value}' but has '${enumerants.join("', '")}'`, undef]
        return ["", value]
    }

    if (type_name.startsWith('.') && (value !== null)) {
        if (['.float32', '.float64'].some((_) => (_ === type_name)) && (typeof value !== 'number'))
            return [`${displayPath(path)}: must be float, not ${JSON.stringify(value)}`, undef]
        if (('.bool' === type_name) && (typeof value !== 'boolean'))
            return [`${displayPath(path)}: must be true or false, not ${JSON.stringify(value)}`, undef]
        if (('.string' === type_name) && (typeof value !== 'string'))
            return [`${displayPath(path)}: must be string, not ${JSON.stringify(value)}`, undef]
        const value_i = ((typeof value === 'number') && (value.toString().includes('.') || value.toString().includes('e')))
            ? Number.NaN : parseInt(value)
        if (['.uint8', '.uint16', '.uint32', '.uint64', '.int8', '.int16', '.int32', '.int64'].some((_) => (_ === type_name)) && ((typeof value !== 'number') || Number.isNaN(value_i)))
            return [`${displayPath(path)}: must be integer, not ${JSON.stringify(value)}`, undef]
        if (['.uint8', '.uint16', '.uint32', '.uint64'].some((_) => (_ === type_name)) && (value_i < 0))
            return [`${displayPath(path)}: must be greater than 0, not ${JSON.stringify(value)}`, undef]
        return ["", value]
    }

    if (is_str && value && !stringIsNoJson)
        try {
            value = JSON.parse(value.toString())
        } catch (err) {
            return [`${err}`, undef]
        }

    if (type_name.startsWith('[') && type_name.endsWith(']') && value) {
        if (!Array.isArray(value))
            return [`${displayPath(path)}: must be null or ${type_name}, not ${value}`, undef]
        for (const i in (value as [])) {
            const item = (value as [])[i]
            const [err_msg, _] = validate(apiRefl, type_name.substring(1, type_name.length - 1), item, path + '[' + i + ']', true)
            if (err_msg && err_msg !== "")
                return [err_msg, undef]
        }
    }

    if (value && (typeof value !== 'object'))
        return [`${displayPath(path)}: must be null or ${type_name}, not ${value}`, undef]

    if (type_name.startsWith('{') && type_name.endsWith('}') && value) {
        const splits = type_name.substring(1, type_name.length - 1).split(':')
        for (const key in (value as object)) {
            const [err_msg_key, _] = validate(apiRefl, splits[0], key, path + '["' + key + '"]', true)
            if (err_msg_key && err_msg_key !== "")
                return [err_msg_key, undef]
            const val = value[key]
            const [err_msg_val, __] = validate(apiRefl, splits.slice(1).join(':'), val, path + '["' + key + '"]', true)
            if (err_msg_val && err_msg_val !== "")
                return [err_msg_val, undef]
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
                return [`${displayPath(path, k)}: '${type_name}' has no '${k}' but has: '${type_struc_field_names.join("', '")}'`, undef]
            const [err_msg, _] = validate(apiRefl, field_type_name, (value as object)[k], path + '.' + k, true)
            if (err_msg !== '')
                return [err_msg, undef]
        }
    }
    return ["", value]
}

function fieldInputValue(v: any, preserve: boolean) {
    return (((typeof v === 'boolean') && (v || preserve))
        || ((typeof v === 'number') && !isNaN(v) && ((v !== 0) || preserve))
        || ((typeof v === 'string') && ((v !== '') || preserve))
        || ((v === null) && preserve)
    ) ? v : (v ? v : undef)
}

function numTypeMin(typeName: string): number { return numTypeLimits(typeName)[0] }
function numTypeMax(typeName: string): number { return numTypeLimits(typeName)[1] }
function numTypeLimits(typeName: string): [number, number] {
    switch (typeName) {
        case '.int8': return [-128, 127]
        case '.int16': return [-32768, 32767]
        case '.int32': return [-2147483648, 2147483647]
        case '.int64': return [-9007199254740991, 9007199254740991] // JS limits, not i64 limits
        case '.uint8': return [0, 255]
        case '.uint16': return [0, 65535]
        case '.uint32': return [0, 4294967295]
        case '.uint64': return [0, 9007199254740991] // JS limits, not i64 limits
    }
    return [Number.MIN_SAFE_INTEGER, Number.MAX_SAFE_INTEGER] // JS limits: +-9007199254740991
}
