import van from './vanjs/van-1.2.3.debug.js'
import * as util from './util.js'


const undef = void 0
const html = van.tags


type YoReflType = { [_: string]: string }

type YoReflApis = {
    Methods: YoReflMethod[]
    Types: { [_: string]: YoReflType }
    Enums: { [_: string]: string[] }
    DbStructs: string[]
}

type YoReflMethod = {
    In: string
    Out: string
    Path: string
}

export function onInit(parent: HTMLElement, apiRefl: YoReflApis, yoReq: (methodPath: string, payload: any, form?: FormData, urlQueryArgs?: { [_: string]: string }) => Promise<any>, getCurUser: () => string) {
    let select_method: HTMLSelectElement, select_history: HTMLSelectElement, td_input: HTMLTableCellElement, td_output: HTMLTableCellElement,
        table: HTMLTableElement, input_query_args: HTMLInputElement, textarea_payload: HTMLTextAreaElement, textarea_response: HTMLTextAreaElement,
        tree_payload: HTMLUListElement, tree_response: HTMLUListElement, div_validate_error_msg: HTMLDivElement
    let last_textarea_payload_value = ''
    const state_email_addr_default = '(none)', state_email_addr = van.state(getCurUser() || state_email_addr_default)
    const auto_completes: { [_: string]: HTMLDataListElement } = {}
    const refreshAutoCompletes = () => {
        for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i)!
            if (key.startsWith('yo.s:')) {
                const json_strs = localStorage.getItem(key)!
                const strs = JSON.parse(json_strs) as string[]
                const str_name = key.substring('yo.s:'.length)
                let is_new = false, datalist = auto_completes[str_name]
                if (!datalist) {
                    [is_new, datalist] = [true, html.datalist({ 'id': 'autocomplete_' + str_name })]
                    auto_completes[str_name] = datalist
                }
                datalist.replaceChildren(...strs.map((_) => html.option({ 'value': _ })))
                if (is_new)
                    van.add(document.body, datalist)
            }
        }
    }

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
            input_query_args.value = ''
            return buildApiMethodGui(true)
        }
        const date_time = parseInt(select_history.selectedOptions[0].value), method_path = select_method.selectedOptions[0].value
        const entries = historyOf(method_path), method = apiRefl.Methods.find((_) => (_.Path === method_path))
        if (method)
            for (const entry of entries)
                if (entry.dateTime === date_time) {
                    input_query_args.value = (entry.queryString ? JSON.stringify(entry.queryString) : '')
                    textarea_payload.value = JSON.stringify(entry.payload, null, 2)
                    onPayloadTextAreaMaybeModified(method_path, method.In)
                    break
                }
    }

    const onPayloadTextAreaMaybeModified = (methodPath: string, typeName: string) => {
        if (textarea_payload.value === last_textarea_payload_value)// not every keyup is a modify
            return
        last_textarea_payload_value = textarea_payload.value
        const [err_msg, obj] = validate(apiRefl, typeName, textarea_payload.value, typeName)
        div_validate_error_msg.innerText = err_msg ? util.strReplace(err_msg, '\n', ' ') : '(all ok)' /* no empty string or whitespace-only, as that'd (visually) `display:none` the div and its outer td/tr */
        div_validate_error_msg.style.visibility = (((err_msg || '').length > 0) ? 'visible' : 'hidden')
        refreshTree(methodPath, obj, tree_payload, true)
        if (textarea_response)
            textarea_response.style.backgroundColor = '#f0f0f0'
    }

    const buildApiTypeGui = (td: HTMLTableCellElement, isForPayload: boolean, type_name: string) => {
        const method_path = select_method.selectedOptions[0].value
        const on_textarea_maybe_modified = () => {
            if (isForPayload)
                onPayloadTextAreaMaybeModified(method_path, type_name)
        }
        const tree = html.ul({ 'style': 'font-size:0.88em' }), textarea = html.textarea({ 'class': 'src-json', 'readOnly': !isForPayload, 'onkeyup': on_textarea_maybe_modified, 'onpaste': on_textarea_maybe_modified, 'oncut': on_textarea_maybe_modified, 'onchange': on_textarea_maybe_modified }, '')
        if (isForPayload)
            [textarea_payload, tree_payload] = [textarea, tree]
        else
            [textarea_response, tree_response] = [textarea, tree]
        if (type_name && type_name !== '') {
            const dummy_val = newSampleVal(apiRefl, type_name, [], isForPayload, true, isForPayload ? method_path : undef)
            textarea.value = JSON.stringify(dummy_val, null, 2)

            on_textarea_maybe_modified()
            refreshTree(method_path, dummy_val, isForPayload ? tree_payload : tree_response, isForPayload)
            td.replaceChildren(textarea, tree)
        } else
            td.replaceChildren()
    }

    const buildApiMethodGui = (noHistorySelect?: boolean) => {
        if (!noHistorySelect)
            refreshHistory(true, false)
        const method_path = select_method.selectedOptions[0].value
        document.title = "/" + method_path
        const method = apiRefl.Methods.find((_) => (_.Path === method_path))!
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
        if (isForPayload)
            last_textarea_payload_value = textarea.value
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
                if (isForPayload)
                    last_textarea_payload_value = textarea.value
                textarea.setSelectionRange(text_sel_start, text_sel_end, "backward")
            }
        }
        return false
    }

    const refreshTree = (methodPath: string, obj: object | null, ulTree: HTMLUListElement, isForPayload: boolean) => {
        const method = apiRefl.Methods.find((_) => (_.Path === methodPath))!
        const type_name = (isForPayload ? method.In : method.Out)
        refreshTreeNode(type_name, obj, ulTree, isForPayload, '', obj)
    }

    const refreshTreeNode = (typeName: string, value: any, ulTree: HTMLUListElement, isForPayload: boolean, path: string, root: any) => {
        while (typeName.startsWith('?'))
            typeName = typeName.substring(1)
        const type_struc = apiRefl.Types[typeName], is_array = Array.isArray(value)
        let ret_count = -1
        ulTree.replaceChildren()
        if (!value)
            return ret_count
        const buildItemInput = (itemTypeName: string, key: string, val: any) => {
            const is_opt = itemTypeName.startsWith('?')
            while (itemTypeName.startsWith('?'))
                itemTypeName = itemTypeName.substring(1)
            let field_input: HTMLElement | null = null, checkbox: HTMLInputElement, get_val: undefined | ((_: string) => any) = undef
            const on_change = (evt: UIEvent) => {
                const is_checkbox_change = (evt.currentTarget === checkbox)
                let index: string | number = key, refresh_tree = false
                if (key.startsWith('["') && key.endsWith('"]'))
                    index = key.substring(2, key.length - 2)
                else if (key.startsWith('[') && key.endsWith(']') && is_array)
                    index = parseInt(key.substring(1))
                if (!get_val) {
                    const sub_val = fieldInputValue(value[index], is_array)
                    if ((checkbox.checked) && ((sub_val === null) || (sub_val === undef))) {
                        const new_val = fieldInputValue(newSampleVal(apiRefl, itemTypeName, [], false, false), is_array || is_checkbox_change || is_opt)
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
                    else if (is_array && (typeof index === 'number'))
                        value[index] = null
                    else {
                        const new_val = fieldInputValue(newSampleVal(apiRefl, itemTypeName, [], isForPayload, false), is_array || is_checkbox_change || is_opt)
                        if (new_val === undef)
                            checkbox.checked = false
                        else
                            [refresh_tree, value[index]] = [true, new_val]
                    }
                }
                if (field_input)
                    if (get_val) // have a field input control
                        field_input.style.visibility = (checkbox.checked ? 'visible' : 'hidden')
                    else // sub-tree
                        field_input.style.display = (checkbox.checked ? 'block' : 'none')
                if (!checkbox.checked)
                    if (typeof index === 'number')
                        value[index] = null
                    else
                        delete value[index]
                const textarea = (isForPayload ? textarea_payload : textarea_response)
                const textarea_newvalue = JSON.stringify(root, null, 2)
                const dim_textarea_response = isForPayload && (textarea_newvalue != textarea.value)
                textarea.value = textarea_newvalue
                if (isForPayload)
                    last_textarea_payload_value = textarea.value
                if (dim_textarea_response)
                    textarea_response.style.backgroundColor = '#f0f0f0'
                selJsonFromTree(path + '.' + key, isForPayload, !refresh_tree)
                if (refresh_tree)
                    refreshTreeNode(typeName, value, ulTree, isForPayload, path, root)
            }
            if (isDtType(itemTypeName) && (typeof val === 'string') && (val.length >= 16) && !Number.isNaN(Date.parse(val))) {
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
                field_input = html.input({ 'onchange': on_change, 'type': 'text', 'list': 'autocomplete_' + autoCompleteFieldName(util.strTrimL(path + '.' + key, '.').split('.')), 'readOnly': !isForPayload, 'value': val })
                get_val = (s: string) => s
            } else if ((itemTypeName === '.bool') && (typeof val === 'boolean')) {
                field_input = html.input({ 'onchange': on_change, 'type': 'checkbox', 'disabled': !isForPayload, 'checked': val })
                get_val = (_: string) => (field_input as HTMLInputElement).checked
            } else if (enumExists(apiRefl, itemTypeName) && (typeof val === 'string')) {
                const enumerants = apiRefl.Enums[itemTypeName]
                if (enumerants && (enumerants.length > 0) && ((enumerants.indexOf(val) >= 0) || (val === '')))
                    field_input = html.select({ 'onchange': on_change }, ...[html.option({ 'value': '', 'selected': (val === '') }, "")].concat(enumerants.map((_) => html.option({ 'value': _, 'selected': (_ === val) }, _))))
                else
                    field_input = html.input({ 'onchange': on_change, 'type': 'text', 'readOnly': !isForPayload, 'value': val })
                get_val = (s: string) => s
            }
            let sub_count = -1, field_input_subs = false, field_input_got = (field_input ? true : false)
            if (field_input_subs = (!field_input_got)) {
                field_input = html.ul({})
                sub_count = refreshTreeNode(itemTypeName, val, field_input as HTMLUListElement, isForPayload, path + '.' + key, root)
                if (field_input_got = (field_input.innerHTML !== ''))
                    field_input.style.borderStyle = 'solid'
            }
            const on_count_click = (evt: UIEvent) => {
                let sub_key = key
                if (sub_key.startsWith('["') && sub_key.endsWith('"]'))
                    sub_key = sub_key.substring(2, sub_key.length - 2)
                else if (sub_key.startsWith('[') && sub_key.endsWith(']'))
                    sub_key = sub_key.substring(1, sub_key.length - 1)
                const coll = value[sub_key]
                if (coll) {
                    if (Array.isArray(coll)) {
                        const item_type_name = itemTypeName.substring(1, itemTypeName.length - 1)
                        coll.push(newSampleVal(apiRefl, item_type_name, [], isForPayload, false))
                    } else {
                        const prompt_text = "keep entering keys until done:", new_map_keys: string[] = []
                        let last_err_msg = ""
                        while (true) {
                            const new_map_key = prompt(last_err_msg ? last_err_msg : prompt_text, "")
                            if (new_map_key === "")
                                break
                            if (!new_map_key)
                                return false
                            const fake = {} as any
                            fake[new_map_key] = null
                            const [err_msg, _] = validate(apiRefl, itemTypeName, fake, '')
                            if ((!err_msg) && (coll[new_map_key] || (new_map_keys.indexOf(new_map_key) >= 0)))
                                last_err_msg = `already have '${new_map_key}', ${prompt_text}`
                            else if (!(last_err_msg = err_msg))
                                new_map_keys.push(new_map_key)
                        }
                        if (new_map_keys.length > 0) {
                            let item_type_name = itemTypeName.substring(1, itemTypeName.length - 1)
                            item_type_name = item_type_name.substring(item_type_name.indexOf(':') + 1)
                            for (const key of new_map_keys)
                                if (!coll[key])
                                    coll[key] = newSampleVal(apiRefl, item_type_name, [], isForPayload, false)
                        }
                    }
                    refreshTreeNode(typeName, value, ulTree, isForPayload, path, root)
                    on_change(evt)
                }
                return false
            }
            van.add(ulTree, html.li({ 'title': displayPath(path, key) },
                checkbox = html.input({ 'onchange': on_change, 'type': 'checkbox', 'disabled': !isForPayload, 'checked': field_input_got }),
                html.a({ 'class': 'label', 'style': (field_input_subs ? 'width:auto' : ''), 'onclick': () => selJsonFromTree(path + '.' + key, isForPayload) },
                    (key.startsWith('[') ? "" : ".") + key,
                    (sub_count < 0) ? [] : [html.b({ 'class': 'count', 'onclick': on_count_click }, `${itemTypeName.substring(0, 1)}${sub_count}×${itemTypeName.substring(itemTypeName.length - 1)}`)]),
                field_input,
            ))
        }

        const is_arr = (typeName.startsWith('[') && typeName.endsWith(']')), is_map = (typeName.startsWith('{') && typeName.endsWith('}') && typeName.includes(':'))
        if (is_arr || is_map) {
            if ((is_arr && !is_array) || (is_map && (typeof value !== 'object')))
                return ret_count
            ret_count = 0
            for (const key in value) {
                ret_count++
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
        return ret_count
    }

    const sendRequest = async () => {
        const show_err = (err: any) => {
            textarea_response.style.backgroundColor = '#f0d0c0'
            textarea_response.value = `${err}`
            refreshTree(method_path, null, tree_response, false)
        }
        textarea_response.value = "..."
        textarea_response.style.backgroundColor = '#f0f0f0'
        let query_string: { [_: string]: string } | undefined, payload: object, is_validate_failed = false
        if (input_query_args.value && input_query_args.value.length)
            try { query_string = JSON.parse(input_query_args.value) } catch (err) {
                return show_err(`URL query-string object:\n${err}`)
            }
        const method_path = select_method.selectedOptions[0].value
        try {
            const method = apiRefl.Methods.find((_) => (_.Path == method_path))!
            const [err_msg, _] = validate(apiRefl, method.In, payload = JSON.parse(textarea_payload.value), '')
            if (is_validate_failed = (err_msg !== "")) {
                show_err(err_msg)
                if (!confirm("failed to validate, send anyway?"))
                    return
            }
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

        let time_started: number
        const on_done = () => {
            const duration_ms = Date.now() - time_started
            document.title = `${duration_ms}ms`
            state_email_addr.val = getCurUser() || state_email_addr_default
        }
        if (!is_validate_failed) {
            historyStore(apiRefl, method_path, payload, query_string)
            refreshHistory(true, false)
            refreshAutoCompletes()
        }
        try {
            time_started = Date.now()
            const result = await yoReq(method_path, payload, undef, query_string)
            on_done()
            if (!is_validate_failed)
                textarea_response.style.backgroundColor = '#c0f0c0'
            textarea_response.value = JSON.stringify(result, null, 2)
            refreshTree(method_path, result, tree_response, false)
        } catch (err) {
            on_done()
            show_err(JSON.stringify(err, null, 2))
        }
    }

    const openInNewDialog = () => {
        const dialog = html.dialog({ 'style': 'width:88%' })
        van.add(parent, dialog)
        dialog.onclose = () => { dialog.remove() }
        onInit(dialog, apiRefl, yoReq, getCurUser)
        dialog.showModal()
    }

    const now = Date.now().toString()
    van.add(parent,
        html.div({},
            select_method = html.select({ 'autofocus': true, 'onchange': (evt: UIEvent) => buildApiMethodGui() },
                ...[html.option({ 'value': '' }, '')].concat(apiRefl.Methods.map((_) => {
                    return html.option({ 'value': _.Path }, _.Path)
                }))),
            html.button({ 'style': 'margin-left:1em', 'onclick': openInNewDialog }, 'New Dialog...'),
            select_history = html.select({ 'style': 'max-width:44%;float:right', 'onchange': onSelectHistoryItem }, html.option({ 'value': '' }, '')),
        ),
        html.div({}, table = html.table({ 'width': '99%', 'style': 'visibility:hidden' },
            html.tr({}, html.td({ 'colspan': '2', 'style': 'text-align:center', 'align': 'center' },
                html.hr(),
                html.span({ 'class': 'nobr' },
                    html.datalist({ 'id': ('ql' + now) },
                        html.option({ value: '{"yoUser":"fooN@bar.baz"}' }, "?yoUser"),
                        html.option({ value: '{"yoValiOnly":true}' }, "?yoValiOnly"),
                        html.option({ value: '{"yoFail":true}' }, "?yoFail"),
                    ),
                    html.label({ 'for': ('q' + now) }, "URL query args:"),
                    input_query_args = html.input({ 'type': 'text', 'id': ('q' + now), 'list': ('ql' + now), 'value': '', 'placeholder': '{"name":"val", ...}', 'style': 'min-width: 44%; max-width: 88%' }),
                ),
                html.span({ 'class': 'nobr' },
                    html.span({ 'style': 'margin-left: 0.44em' }, "Current login: ", html.b({ 'style': 'margin-right: 0.44em' }, state_email_addr)),
                ),
                html.button({ 'style': 'font-weight:bold', 'onclick': sendRequest }, 'Go!'),
            )),
            html.tr({}, html.td({ 'colspan': '2' }, div_validate_error_msg = html.div({ 'style': 'background-color:#f0d0c0;border: 1px solid #303030;visibility:hidden' }, 'some error message'))),
            html.tr({},
                td_input = html.td({ 'width': '50%' }),
                td_output = html.td({ 'width': '50%' }),
            ),
        )),
    )
    historyCleanUp(apiRefl)
    refreshHistory(false, false)
    refreshAutoCompletes()
    const entry = historyLatest()
    for (let i = 0; entry && i < select_method.options.length; i++) {
        if (select_method.options[i].value === entry.methodPath) {
            select_method.selectedIndex = i
            buildApiMethodGui()
            break
        }
    }
}

function newSampleVal(refl: YoReflApis, type_name: string, recurse_protection: string[], isForPayload: boolean, isRootVal: boolean, methodPath?: string): any {
    switch (type_name) {
        case 'time.Time': case 'yo/db.DateTime': return isForPayload ? null : new Date().toISOString()
        case '.bool': return isForPayload ? false : true
        case '.string': return isForPayload ? "" : "foo bar"
        case '.float32': return isForPayload ? 0.0 : 3.2
        case '.float64': return isForPayload ? 0.0 : 6.4
        case '.int8': return isForPayload ? 0 : -8
        case '.int16': return isForPayload ? 0 : -16
        case '.int32': return isForPayload ? 0 : -32
        case '.int64': return isForPayload ? 0 : -64
        case '.uint8': return isForPayload ? 0 : 8
        case '.uint16': return isForPayload ? 0 : 16
        case '.uint32': return isForPayload ? 0 : 32
        case '.uint64': return isForPayload ? 0 : 64
    }

    if (enumExists(refl, type_name)) {
        const enumerants = refl.Enums[type_name]
        if (enumerants && (enumerants.length > 0))
            return isForPayload ? "" : enumerants[Math.floor(Math.random() * enumerants.length)]
        return isForPayload ? "" : `(some ${type_name} enumerant)`
    }

    const is_db_create = methodPath && methodPath.startsWith('__/yo/db/') && (methodPath.endsWith('/createOne') || methodPath.endsWith('/createMany'))
    const type_struc = refl.Types[type_name]
    if (type_struc) {
        const obj: { [_: string]: any } = {}
        if (recurse_protection.indexOf(type_name) >= 0)
            return null
        for (const field_name in type_struc) {
            const field_type_name = type_struc[field_name]
            obj[field_name] = newSampleVal(refl, field_type_name, [type_name].concat(recurse_protection), isForPayload, isRootVal, methodPath)
        }
        const filter_dbstruct_fields = (refl.DbStructs.indexOf(type_name) >= 0) && is_db_create
        if (filter_dbstruct_fields)
            for (const field_name of ['ID', 'Id', 'Created'])
                delete obj[field_name]
        return obj
    }
    if (type_name.startsWith('?'))
        return (isForPayload && !isRootVal) ? null : newSampleVal(refl, type_name.substring(1), recurse_protection, isForPayload, isRootVal, methodPath)
    if (type_name.startsWith('[') && type_name.endsWith(']'))
        return [newSampleVal(refl, type_name.substring(1, type_name.length - 1), recurse_protection, isForPayload, false, methodPath)]
    if (type_name.startsWith('{') && type_name.endsWith('}') && type_name.includes(':')) {
        const ret: { [_: string]: any } = {}, splits = type_name.substring(1, type_name.length - 1).split(':')
        ret[newSampleVal(refl, splits[0], recurse_protection, isForPayload, false, methodPath) as string] = newSampleVal(refl, splits.slice(1).join(':'), recurse_protection, isForPayload, false, methodPath)
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
    const json_entries = localStorage.getItem('yo.h:' + methodPath)
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
        const key = localStorage.key(i)!
        if (!key.startsWith('yo.h:'))
            continue
        const method_path = key.substring('yo.h:'.length)
        const json_entries = localStorage.getItem(key)!
        const entries: HistoryEntry[] = JSON.parse(json_entries)
        for (const entry of entries)
            if ((!ret) || (entry.dateTime > ret.dateTime))
                ret = { dateTime: entry.dateTime, methodPath: method_path, payload: entry.payload, queryString: entry.queryString }
    }
    return ret
}

function historyCleanUp(apiRefl: YoReflApis, newDueToStoreMethodPath?: string, newDueToStoreHistoryEntry?: HistoryEntry) {
    const keys_to_remove: string[] = []
    for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i)!
        if (!key.startsWith('yo.')) {
            keys_to_remove.push(key)
            continue
        }
        if (key.startsWith('yo.h:')) {
            const method_path = key.substring('yo.h:'.length)
            if (!apiRefl.Methods.some((_) => (_.Path === method_path))) // methodPath no longer part of API
                keys_to_remove.push(key)
            else {
                let mut = false, entries: HistoryEntry[] = JSON.parse(localStorage.getItem(key)!)
                for (let i = 0; i < entries.length; i++) {
                    // check for equality with current payload/queryString: anything the same can go
                    const entry = entries[i], method = apiRefl.Methods.find((_) => (_.Path === method_path))!
                    const remove = ('' !== validate(apiRefl, method.In, entry.payload, method.In)[0]) ||
                        (newDueToStoreMethodPath && newDueToStoreHistoryEntry && (newDueToStoreMethodPath === method_path) && util.deepEq(entry.payload, newDueToStoreHistoryEntry.payload) && util.deepEq(entry.queryString, newDueToStoreHistoryEntry.queryString))
                    if (remove)
                        [mut, i, entries] = [true, i - 1, entries.filter((_) => (_ != entry))]
                }
                if (mut)
                    localStorage.setItem(key, JSON.stringify(entries))
            }
        }
        if (key.startsWith('yo.s:')) {
            const json_strs = localStorage.getItem(key)
            if ((!json_strs) || (json_strs.length) <= 2) // JSON value of [] stored. cannot actually happen but whatev
                keys_to_remove.push(key)
            else {
                const str_name = key.substring('yo.s:'.length)
                let field_name_still_exists = false
                for (const type_ident in apiRefl.Types) {
                    const type_refl = apiRefl.Types[type_ident]
                    if (type_refl)
                        for (const field_name in type_refl)
                            if (field_name_still_exists = (field_name === str_name))
                                break
                    if (field_name_still_exists)
                        break
                }
                if (!field_name_still_exists)
                    keys_to_remove.push(key)
            }
        }
    }
    for (const key_to_remove of keys_to_remove)
        localStorage.removeItem(key_to_remove)
}

function historyStore(apiRefl: YoReflApis, methodPath: string, payload: object, queryString?: object) {
    const entry: HistoryEntry = {
        dateTime: Date.now(),
        payload: payload,
        queryString: queryString
    }

    historyCleanUp(apiRefl, methodPath, entry)  // since we're anyway writing to localStorage, a good moment to clean out no-longer-needed history entries

    const method = apiRefl.Methods.find((_) => (_.Path === methodPath))!

    let json_entries = localStorage.getItem('yo.h:' + methodPath)
    if (!(json_entries && json_entries.length))
        json_entries = '[]'
    let entries: HistoryEntry[] = JSON.parse(json_entries)
    entries.push(entry)
    json_entries = JSON.stringify(entries)
    let not_stored_yet = true
    while (not_stored_yet)
        try {
            localStorage.setItem('yo.h:' + methodPath, json_entries)
            not_stored_yet = false
        } catch (err) {
            if (entries.length === 0) {
                console.error(err)
                break
            }
            entries = entries.slice(1)
        }

    walk(apiRefl, method.In, entry.payload, [], (path, fieldTypeName, fieldValue) => {
        while (fieldTypeName.startsWith('?'))
            fieldTypeName = fieldTypeName.substring(1)
        if ((fieldTypeName !== '.string') || (typeof fieldValue !== 'string') || !fieldValue)
            return
        const field_name = autoCompleteFieldName(path)
        if (field_name) {
            const storage_key = 'yo.s:' + field_name
            const json_entries = localStorage.getItem(storage_key) || '[]'
            const entries = JSON.parse(json_entries) as string[]
            if (entries.indexOf(fieldValue) < 0) {
                entries.push(fieldValue)
                localStorage.setItem(storage_key, JSON.stringify(entries))
            }
        }
    })
}

function autoCompleteFieldName(path: string[]): string {
    if (path.length)
        for (let i = path.length - 1; i >= 0; i--) {
            const path_item = path[i]
            if (!(path_item.startsWith('[') && path_item.endsWith(']')))
                return path_item
        }
    return ''
}

function displayPath(p: string, k?: string) {
    return p + (k ? ("." + k) : "")
}

function enumExists(apiRefl: YoReflApis, type_name: string) {
    return (Object.keys(apiRefl.Enums).indexOf(type_name) >= 0)
}

function walk(apiRefl: YoReflApis, typeName: string, value: any, path: string[], onField: (path: string[], fieldTypeName: string, fieldValue: any) => void, callForArrsAndObjs: boolean = false) {
    while (typeName.startsWith('?'))
        typeName = typeName.substring(1)

    if (typeName.startsWith('[') && typeName.endsWith(']') && value && Array.isArray(value)) {
        const type_name_items = typeName.substring(1, typeName.length - 1)
        for (let i = 0; i < value.length; i++)
            walk(apiRefl, type_name_items, value[i], path.concat([`[${i}]`]), onField, callForArrsAndObjs)
        if (!callForArrsAndObjs)
            return
    }

    if (typeName.startsWith('{') && typeName.endsWith('}') && typeName.includes(':') && value) {
        const splits = typeName.substring(1, typeName.length - 1).split(':')
        const type_name_val = splits.slice(1).join(':')
        for (const key in value)
            walk(apiRefl, type_name_val, value[key], path.concat([`["${key}"]`]), onField, callForArrsAndObjs)
        if (!callForArrsAndObjs)
            return
    }

    const type_struc = apiRefl.Types[typeName]
    if (type_struc && value) {
        for (const field_name in value) {
            const field_type_name = type_struc[field_name]
            if (field_type_name)
                walk(apiRefl, field_type_name, value[field_name], path.concat([field_name]), onField, callForArrsAndObjs)
        }
        if (!callForArrsAndObjs)
            return
    }

    onField(path, typeName, value)
}

function validate(apiRefl: YoReflApis, typeName: string, value: any, path: string, stringIsNoJson?: boolean): [string, any] {
    while (typeName.startsWith('?'))
        typeName = typeName.substring(1)

    const is_str = (typeof value === 'string')
    if (value === undef)
        return [`${displayPath(path)}: new bug, 'value' being 'undefined'`, undef]

    if (isDtType(typeName)) {
        if (!((is_str && value !== '') || (value === null)))
            return [`${displayPath(path)}: must be non-empty string or null`, undef]
        else if (is_str && value && Number.isNaN(Date.parse(value.toString())))
            return [`${displayPath(path)}: must be 'Date.parse'able`, undef]
        return ["", value]
    }

    if (enumExists(apiRefl, typeName)) {
        const enumerants = apiRefl.Enums[typeName]
        if (enumerants && (enumerants.length > 0) && value && (enumerants.indexOf(value) < 0))
            return [`${displayPath(path)}: '${typeName}' has no '${value}' but has '${enumerants.join("', '")}'`, undef]
        return ["", value]
    }

    if (typeName.startsWith('.') && (value !== null)) {
        const [min, max] = (typeName.startsWith('.u') || typeName.startsWith('.i') || typeName.startsWith('.f')) ? numTypeLimits(typeName) : [0, 0]
        if (['.float32', '.float64'].some((_) => (_ === typeName)) && ((typeof value !== 'number') || Number.isNaN(value) || (value < min) || (value > max)))
            return [`${displayPath(path)}: must be ${typeName}, not ${JSON.stringify(value)}`, undef]
        if (('.bool' === typeName) && (typeof value !== 'boolean'))
            return [`${displayPath(path)}: must be true or false, not ${JSON.stringify(value)}`, undef]
        if (('.string' === typeName) && (typeof value !== 'string'))
            return [`${displayPath(path)}: must be string, not ${JSON.stringify(value)}`, undef]
        const value_i = ((typeof value === 'number') && (value.toString().includes('.') || value.toString().includes('e')))
            ? Number.NaN : parseInt(value)
        if (['.uint8', '.uint16', '.uint32', '.uint64', '.int8', '.int16', '.int32', '.int64'].some((_) => (_ === typeName)) && ((typeof value !== 'number') || Number.isNaN(value_i) || (value_i < min) || (value_i > max)))
            return [`${displayPath(path)}: must be ${typeName}, not ${JSON.stringify(value)}`, undef]
        return ["", value]
    }

    if (is_str && value && !stringIsNoJson)
        try {
            value = JSON.parse(value.toString())
        } catch (err) {
            return [`${err}`, undef]
        }

    const str_from = (v: any) => {
        const ret = `${v}`
        return (ret === '[object Object]') ? '{...}' : ret
    }

    if (typeName.startsWith('[') && typeName.endsWith(']') && value) {
        if (!Array.isArray(value))
            return [`${displayPath(path)}: must be null or ${typeName}, not ${str_from(value)}`, undef]
        const type_name_items = typeName.substring(1, typeName.length - 1)
        for (const i in (value as [])) {
            const item = (value as [])[i]
            const [err_msg, _] = validate(apiRefl, type_name_items, item, path + '[' + i + ']', true)
            if (err_msg && err_msg !== "")
                return [err_msg, undef]
        }
    }

    if (value && (typeof value !== 'object'))
        return [`${displayPath(path)}: must be null or ${typeName}, not ${str_from(value)}`, undef]

    if (typeName.startsWith('{') && typeName.endsWith('}') && value) {
        const splits = typeName.substring(1, typeName.length - 1).split(':')
        const type_name_key = splits[0], type_name_val = splits.slice(1).join(':')
        for (const key in (value as object)) {
            const [err_msg_key, _] = validate(apiRefl, type_name_key, key, path + '["' + key + '"]', true)
            if (err_msg_key && err_msg_key !== "")
                return [err_msg_key, undef]
            const val = value[key]
            const [err_msg_val, __] = validate(apiRefl, type_name_val, val, path + '["' + key + '"]', true)
            if (err_msg_val && err_msg_val !== "")
                return [err_msg_val, undef]
        }
    }

    const type_struc = apiRefl.Types[typeName]
    if (type_struc && value) {
        const type_struc_field_names = []
        for (const type_field_name in type_struc)
            type_struc_field_names.push(type_field_name)
        for (const field_name in (value as object)) {
            const field_type_name = type_struc[field_name]
            if (!field_type_name)
                return [`${displayPath(path, field_name)}: '${typeName}' has no '${field_name}'` + ((type_struc_field_names.length < 1) ? " field" : (` but has: '${type_struc_field_names.join("' and '")}'`)), undef]
            const [err_msg, _] = validate(apiRefl, field_type_name, value[field_name], path + '.' + field_name, true)
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
    while (typeName.startsWith('?'))
        typeName = typeName.substring(1)
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
function isDtType(typeName: string): boolean {
    return (typeName === 'time.Time') || (typeName === 'yo/db.DateTime')
}
