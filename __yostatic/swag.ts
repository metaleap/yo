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
    let td_input: HTMLTableCellElement, td_output: HTMLTableCellElement

    const buildApiTypeGui = (td: HTMLTableCellElement, readOnly: boolean, type_name: string) => {
        const dummy_val = buildVal(apiRefl, type_name, [])
        td.innerHTML = ''
        van.add(td, html.textarea({ 'class': 'src-json', 'readOnly': readOnly }, JSON.stringify(dummy_val, null, 2)))
    }
    const buildApiMethodGui = (sel: HTMLSelectElement) => {
        const method = apiRefl.Methods.find((_) => (_.Path === sel.selectedOptions[0].value))
        buildApiTypeGui(td_input, false, method.In)
        buildApiTypeGui(td_output, true, method.Out)
    }

    van.add(document.body,
        html.div({}, html.select({ 'autofocus': true, 'onchange': (evt: UIEvent) => buildApiMethodGui(evt.target as HTMLSelectElement) },
            ...apiRefl.Methods.map((_) => {
                return html.option({ 'value': _.Path }, _.Path)
            }))),
        html.div({}, html.table({ 'width': '99%' }, html.tr({},
            td_input = html.td({ 'width': '50%' }),
            td_output = html.td({ 'width': '50%' }),
        ))),
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
    if (type_name.startsWith('{') && type_name.endsWith('}')) {
        const ret = {}, [tkey, tval] = type_name.substring(1, type_name.length - 1).split(':')
        ret[buildVal(refl, tkey, recurse_protection)] = buildVal(refl, tval, recurse_protection)
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
