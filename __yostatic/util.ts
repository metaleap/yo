import van, { State } from './vanjs/van-1.2.6.js'
const none = void 0

export type Direction = 0 | 1 | -1 | typeof NaN
export const DirPrev: Direction = -1
export const DirNext: Direction = 1
export const DirStart: Direction = 0
export const DirEnd: Direction = NaN

export function arrayCanMove<T>(arr: T[], idxOld: number, direction: Direction): number | undefined {
    if (arr.length < 2)
        return undefined
    const idx_new =
        (direction == DirPrev) ? (idxOld - 1)
            : ((direction == DirNext) ? (idxOld + 1)
                : ((direction == DirStart) ? 0
                    : (arr.length - 1)))
    const can_move = (idx_new != idxOld) && (idx_new >= 0) && (idx_new < arr.length)
    return can_move ? idx_new : undefined
}

export function arrayMoveItem<T>(arr: T[], idxOld: number, idxNew: number): T[] {
    const item = arr[idxOld]
    arr.splice(idxOld, 1)
    arr.splice(idxNew, 0, item)
    return arr
}

export function errStr(err: any) {
    if ((err === undefined) || (err === null))
        return ""
    const err_json = JSON.stringify(err), err_str_1 = err.toString(), err_str_2 = `${err}`
    return err.knownErr || err.message ||
        ((err_str_1 && (err_str_1 !== '[object Object]')) ? err_str_1 :
            ((err_str_2 && (err_str_2 !== '[object Object]')) ? err_str_2
                : err_json))
}

export function fEq(a: number, b: number): boolean {
    return (isNaN(a) || isNaN(b))
        ? (isNaN(a) && isNaN(b))
        : ((a === b) || Math.abs(a - b) < (((a > 1) || (b > 1)) ? (Math.max(a, b) * Number.EPSILON) : Number.EPSILON))
}

export function strTrimL(s: string, prefix: string): string {
    while (s.startsWith(prefix))
        s = s.substring(prefix.length)
    return s
}

export function strReplace(s: string, oldStr: string, newStr: string): string {
    return s.replaceAll(oldStr, newStr)
}

// deepEq only covers the JSON subset of the JS/TS type-scape
export function deepEq(val1: any, val2: any, ignoreArrayOrder?: boolean, dbgPrintDiff?: boolean): boolean {
    if ((val1 === val2) || ((val1 === null) && (val2 === none)) || ((val1 === none) && (val2 === null)))
        return true

    if ((typeof val1) !== (typeof val2)) {
        if (dbgPrintDiff)
            console.log("types:", typeof val1, val1, "!==", typeof val2, val2)
        return false
    }

    if (((typeof val1) === 'number') && ((typeof val2) === 'number')) {
        const is_float_eq = fEq(val1, val2)
        if ((!is_float_eq) && dbgPrintDiff)
            console.log("!fEq", val1, val2)
        return is_float_eq
    }

    if (((typeof val1) === 'object') && ((typeof val2) === 'object')) {
        const is_arr_1 = Array.isArray(val1), is_arr_2 = Array.isArray(val2)

        if (is_arr_1 != is_arr_2) {
            if (dbgPrintDiff)
                console.log("arr vs non-arr:", val1, val2)
            return false
        }

        if (is_arr_1 && is_arr_2 && val1.length != val2.length) {
            if (dbgPrintDiff)
                console.log("arr-lengths:", val1.length, val1, val2.length, val2)
            return false
        }

        if (!(is_arr_1 && is_arr_2)) { // 2 objects
            let len1 = 0, len2 = 0
            for (const _ in val2)
                len2++
            for (const k in val1)
                if ((((++len1) > len2) && !dbgPrintDiff) || !deepEq(val1[k], val2[k], ignoreArrayOrder, dbgPrintDiff)) {
                    if (dbgPrintDiff)
                        console.log("obj@" + k, val1[k], val2[k])
                    return false
                }
            if (len1 !== len2) {
                if (dbgPrintDiff)
                    console.log("obj-lengths:", len1, val1, len2, val2)
                return false
            }
            return true
        }

        if (!ignoreArrayOrder) { // 2 arrays, in order
            for (let i = 0; i < val1.length; i++)
                if (!deepEq(val1[i], val2[i], ignoreArrayOrder, dbgPrintDiff)) {
                    if (dbgPrintDiff)
                        console.log("arr@" + i, val1[i], val2[i])
                    return false
                }
            return true
        }

        { // 2 arrays, ignoring order
            for (const item1 of val1) {
                let found = false
                for (const item2 of val2)
                    if (found = deepEq(item1, item2, ignoreArrayOrder, false))
                        break
                if (!found) {
                    if (dbgPrintDiff)
                        console.log("notFound:", item1, "in", val1, "but not in", val2)
                    return false
                }
            }
            return true
        }
    }

    if (dbgPrintDiff)
        console.log(JSON.stringify(val1) + ':' + (typeof val1) + " !== " + JSON.stringify(val2) + ':' + (typeof val2))
    return false
}



export type DomLive<T extends { [_: string]: any }> = {
    domNode: Element
    all: T[]
    itemCount: State<number>
    timeLastModifiedDomWise: State<number>
    replaceWith: (_: T[]) => void
}

export function domLive<T extends { [_: string]: any }>(domNode: Element, initial: T[], perItem: (_: T) => Element, identPropName = 'Id'): DomLive<T> {
    type _DomLive = DomLive<T> & {
        _lastNodes: { [_: string | number]: Element }
        _lastItems: { [_: string | number]: T }
    }
    const me: _DomLive = {
        _lastNodes: {} as { [_: string | number]: Element },
        _lastItems: {} as { [_: string | number]: T },

        all: initial,
        itemCount: van.state(initial.length),
        timeLastModifiedDomWise: van.state(0),
        domNode: domNode,
        replaceWith: (items: T[]) => {
            let dom_muts = false
            // find dom nodes to remove, then remove them
            const del_nodes: Element[] = []
            for (const id in me._lastItems) {
                if (!items.some(_ => (id == (_[identPropName]!)))) { // no `===` here due to string-vs-number ambiguity
                    const node_old = me._lastNodes[id]
                    del_nodes.push(node_old)
                    delete me._lastNodes[id]
                    delete me._lastItems[id]
                }
            }
            if (dom_muts = (del_nodes.length > 0))
                for (const del_node of del_nodes)
                    del_node.replaceWith()
            // ignoring changes in sort order for now here, actual node-(re)create ops per item (if changed)
            const new_nodes = [] as Element[]
            for (let i = 0, l = items.length; i < l; i++) {
                const item = items[i]
                const item_id = item[identPropName]!
                const node_old = me._lastNodes[item_id], item_old = me._lastItems[item_id]
                if (!deepEq(item, item_old, false, false)) {
                    const node_new = perItem(item)
                    if (!node_old) // new dom append
                        new_nodes.push(node_new)
                    else  // change dom node
                        node_old.replaceWith(node_new)
                    me._lastNodes[item_id] = node_new
                }
                me._lastItems[item_id] = item
            }
            if (new_nodes.length > 0) {
                dom_muts = true
                me.domNode.append(...new_nodes)
            }
            // ensure up-to-date sort order
            for (let i = 0, l = (items.length - 1); i < l; i++) {
                const node_this = me._lastNodes[items[i][identPropName]!], node_next = me._lastNodes[items[i + 1][identPropName]!]
                if (node_this.nextElementSibling !== node_next) {
                    dom_muts = true
                    me.domNode.insertBefore(node_this, node_next)
                }
            }

            me.all = items
            me.itemCount.val = items.length
            if (dom_muts)
                me.timeLastModifiedDomWise.val = Date.now()
        }
    }
    me.replaceWith(initial)
    return me
}
