const none = void 0


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

export function deepEq(val1: any, val2: any, ignoreArrayOrder?: boolean, dbgPrintDiff?: boolean): boolean {
    // deepEq only covers the JSON subset of the JS/TS type-scape
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
                    if (found = deepEq(item1, item2, ignoreArrayOrder, dbgPrintDiff))
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
