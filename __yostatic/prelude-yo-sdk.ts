export type I8 = number
export type I16 = number
export type I32 = number
export type I64 = number
export type U8 = number
export type U16 = number
export type U32 = number
export type U64 = number
export type F32 = number
export type F64 = number


export let userEmailAddr = ''
export let reqTimeoutMilliSecForJsonApis = 4321
export let reqTimeoutMilliSecForMultipartForms = 54321
export let reqMaxReqPayloadSizeMb = 0           // declaration only, generated code sets the value
export let errMaxReqPayloadSizeExceeded = ""    // declaration only, generated code sets the value

export function setReqTimeoutMilliSec(reqTimeoutMsForJsonApis: number, reqTimeoutMsForMultipartForms: number) {
    reqTimeoutMilliSecForJsonApis = reqTimeoutMsForJsonApis
    reqTimeoutMilliSecForMultipartForms = reqTimeoutMsForMultipartForms
}

export async function req<TIn, TOut, TErr extends string>(methodPath: string, payload?: TIn | {}, formData?: FormData, urlQueryArgs?: { [_: string]: string }): Promise<TOut> {
    let rel_url = '/' + methodPath
    if (urlQueryArgs)
        rel_url += ('?' + new URLSearchParams(urlQueryArgs).toString())

    if (!payload)
        payload = {}
    const payload_json = JSON.stringify(payload)

    if ((reqMaxReqPayloadSizeMb > 0) && errMaxReqPayloadSizeExceeded && (payload_json.length > (1024 * 1024 * reqMaxReqPayloadSizeMb)))
        throw new Err<TErr>(errMaxReqPayloadSizeExceeded as TErr)

    if (formData) {
        formData.set("_", payload_json)
        if ((reqMaxReqPayloadSizeMb > 0) && errMaxReqPayloadSizeExceeded) {
            let req_payload_size = 0
            formData.forEach(_ => {
                const value = _.valueOf()
                const file = value as File
                if (typeof value === 'string')
                    req_payload_size += value.length
                else if (file && file.name && file.size && (typeof file.size === 'number') && (file.size > 0))
                    req_payload_size += file.size
            })
            if (req_payload_size > (1024 * 1024 * reqMaxReqPayloadSizeMb))
                throw new Err<TErr>(errMaxReqPayloadSizeExceeded as TErr)
        }
    }

    const resp = await fetch(rel_url, {
        method: 'POST', headers: (formData ? undefined : ({ 'Content-Type': 'application/json' })), body: (formData ? formData : payload_json),
        cache: 'no-store', mode: 'same-origin', redirect: 'error', signal: AbortSignal.timeout(formData ? reqTimeoutMilliSecForMultipartForms : reqTimeoutMilliSecForJsonApis),
    })
    if (resp.status !== 200) {
        let body_text: string = '', body_err: any
        try { body_text = await resp.text() } catch (err) { body_err = err }
        throw ({ 'status_code': resp?.status, 'status_text': resp?.statusText, 'body_text': body_text.trim(), 'body_err': body_err })
    }
    userEmailAddr = resp?.headers?.get('X-Yo-User') ?? ''
    return (await resp.json()) as TOut
}

export class Err<T extends string> extends Error {
    knownErr: T
    constructor(err: T) {
        super()
        this.knownErr = err
    }
}

type QueryOperator = 'EQ' | 'NE' | 'LT' | 'LE' | 'GT' | 'GE' | 'IN' | 'AND' | 'OR' | 'NOT'

export interface QueryVal {
    __yoQLitValue?: any,
    __yoQFieldName?: any
    toApiQueryExpr: () => object | null,
}

export class QueryExpr {
    __yoQOp: QueryOperator
    __yoQConds: QueryExpr[] = []
    __yoQOperands: QueryVal[] = []
    private constructor() { }
    and(...conds: QueryExpr[]): QueryExpr { return qAll(...[this as QueryExpr].concat(conds)) }
    or(...conds: QueryExpr[]): QueryExpr { return qAny(...[this as QueryExpr].concat(conds)) }
    not(): QueryExpr { return qNot(this as QueryExpr) }
    toApiQueryExpr(): object {
        const ret = {} as any
        if (this.__yoQOp === 'NOT')
            ret['NOT'] = this.__yoQConds[0].toApiQueryExpr()
        else if ((this.__yoQOp === 'AND') || (this.__yoQOp === 'OR'))
            ret[this.__yoQOp] = this.__yoQConds.map((_) => _.toApiQueryExpr())
        else
            ret[this.__yoQOp] = this.__yoQOperands.map((_) => _.toApiQueryExpr())
        return ret
    }
}

export class QVal<T extends (string | number | boolean | null)>  {
    __yoQLitValue: T
    constructor(literalValue: T) { this.__yoQLitValue = literalValue }
    equal(other: QueryVal): QueryExpr { return qEqual(this, other) }
    notEqual(other: QueryVal): QueryExpr { return qNotEqual(this, other) }
    lessThan(other: QueryVal): QueryExpr { return qLessThan(this, other) }
    lessOrEqual(other: QueryVal): QueryExpr { return qLessOrEqual(this, other) }
    greaterThan(other: QueryVal): QueryExpr { return qGreaterThan(this, other) }
    greaterOrEqual(other: QueryVal): QueryExpr { return qGreaterOrEqual(this, other) }
    in(...set: QueryVal[]): QueryExpr { return qIn(this, ...set) }
    toApiQueryExpr(): object | null {
        if (typeof this.__yoQLitValue === 'string')
            return { 'Str': this.__yoQLitValue ?? '' }
        if (typeof this.__yoQLitValue === 'number')
            return { 'Int': this.__yoQLitValue ?? 0 }
        if (typeof this.__yoQLitValue === 'boolean')
            return { 'Bool': this.__yoQLitValue ?? '' }
        return null
    }
}

export class QFld<T extends string> {
    __yoQFieldName: T
    constructor(fieldName: T) { this.__yoQFieldName = fieldName }
    equal(other: QueryVal): QueryExpr { return qEqual(this, other) }
    notEqual(other: QueryVal): QueryExpr { return qNotEqual(this, other) }
    lessThan(other: QueryVal): QueryExpr { return qLessThan(this, other) }
    lessOrEqual(other: QueryVal): QueryExpr { return qLessOrEqual(this, other) }
    greaterThan(other: QueryVal): QueryExpr { return qGreaterThan(this, other) }
    greaterOrEqual(other: QueryVal): QueryExpr { return qGreaterOrEqual(this, other) }
    in(...set: QueryVal[]): QueryExpr { return qIn(this, ...set) }
    toApiQueryExpr(): object { return { 'Fld': this.__yoQFieldName } }
}

function qAll(...conds: QueryExpr[]): QueryExpr { return { __yoQOp: 'AND', __yoQConds: conds } as QueryExpr }
function qAny(...conds: QueryExpr[]): QueryExpr { return { __yoQOp: 'OR', __yoQConds: conds } as QueryExpr }
function qNot(cond: QueryExpr): QueryExpr { return { __yoQOp: 'NOT', __yoQConds: [cond] } as QueryExpr }

function qEqual(x: QueryVal, y: QueryVal): QueryExpr { return { __yoQOp: 'EQ', __yoQOperands: [x, y] } as QueryExpr }
function qNotEqual(x: QueryVal, y: QueryVal): QueryExpr { return { __yoQOp: 'NE', __yoQOperands: [x, y] } as QueryExpr }
function qLessThan(x: QueryVal, y: QueryVal): QueryExpr { return { __yoQOp: 'LT', __yoQOperands: [x, y] } as QueryExpr }
function qLessOrEqual(x: QueryVal, y: QueryVal): QueryExpr { return { __yoQOp: 'LE', __yoQOperands: [x, y] } as QueryExpr }
function qGreaterThan(x: QueryVal, y: QueryVal): QueryExpr { return { __yoQOp: 'GT', __yoQOperands: [x, y] } as QueryExpr }
function qGreaterOrEqual(x: QueryVal, y: QueryVal): QueryExpr { return { __yoQOp: 'GE', __yoQOperands: [x, y] } as QueryExpr }
function qIn(x: QueryVal, ...set: QueryVal[]): QueryExpr { return { __yoQOp: 'IN', __yoQOperands: [x].concat(set) } as QueryExpr }
