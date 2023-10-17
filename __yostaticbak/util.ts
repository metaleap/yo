const undef = void 0

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
    return (s as any).replaceAll(oldStr, newStr)
}

export function deepEq(val1: any, val2: any, ignoreArrayOrder?: boolean): boolean {
    // deepEq only covers the JSON subset of the JS/TS type-scape
    if ((val1 === val2) || ((val1 === null) && (val2 === undef)) || ((val1 === undef) && (val2 === null)))
        return true
    if ((typeof val1) !== (typeof val2))
        return false
    if (((typeof val1) == 'number') && ((typeof val2) == 'number'))
        return fEq(val1, val2)
    if (((typeof val1) == 'object') && ((typeof val2) == 'object')) {
        const arr1 = Array.isArray(val1), arr2 = Array.isArray(val2)

        if ((arr1 != arr2) || (arr1 && arr2 && val1.length != val2.length))
            return false

        else if (!(arr1 && arr2)) { // 2 objects
            let len1 = 0, len2 = 0
            for (const _ in val2)
                len2++
            for (const k in val1)
                if (((++len1) > len2) || !deepEq(val1[k], val2[k], ignoreArrayOrder))
                    return false
            return (len1 == len2)

        } else if (!ignoreArrayOrder) { // 2 arrays, in order
            for (let i = 0; i < val1.length; i++)
                if (!deepEq(val1[i], val2[i], ignoreArrayOrder))
                    return false
            return true

        } else { // 2 arrays, ignoring order
            for (const item1 of val1) {
                let found = false
                for (const item2 of val2)
                    if (found = deepEq(item1, item2, ignoreArrayOrder))
                        break
                if (!found)
                    return false
            }
            return true
        }
    }
    return false
}
