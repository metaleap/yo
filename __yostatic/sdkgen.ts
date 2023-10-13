export type Yo_i8 = number
export type Yo_i16 = number
export type Yo_i32 = number
export type Yo_i64 = number
export type Yo_u8 = number
export type Yo_u16 = number
export type Yo_u32 = number
export type Yo_u64 = number
export type Yo_f32 = number
export type Yo_f64 = number

export let yoReq_timeoutMilliSec = 1234

export function setReqTimeoutMilliSec(timeout: number) {
    yoReq_timeoutMilliSec = timeout
}

export async function yoReq<TIn, TOut>(methodPath: string, payload: TIn, urlQueryArgs?: { [_: string]: string }) {
    let uri = "/" + methodPath
    if (urlQueryArgs)
        uri += '?' + new URLSearchParams(urlQueryArgs).toString()
    console.log("callAPI:", uri, payload)
    const resp = await fetch(uri, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
        cache: 'no-cache', mode: 'same-origin', redirect: 'error', signal: AbortSignal.timeout(yoReq_timeoutMilliSec)
    })
    if (resp && (resp.status !== 200)) {
        let body_text: string = '', body_err: any
        try { body_text = await resp.text() } catch (err) { if (err) body_err = err }
        throw ({ 'status_code': resp?.status, 'status_text': resp?.statusText, 'body_text': body_text.trim(), 'body_err': body_err })
    }
    const json_resp = await resp.json()
    return json_resp as TOut
}

export type QueryOp = "EQ" | "NE" | "LT" | "LE" | "GT" | "GE" | "IN" | "AND" | "OR" | "NOT"

export interface QVal {
    __yoQLitValue?: any,
    __yoQFieldName?: any
    toApiQueryExpr: () => object,
}

export class Query {
    __yoQOp: QueryOp
    __yoQConds: Query[]
    __yoQOperands: QVal[]

    and(...conds: Query[]): Query { return qAll(...[this as Query].concat(conds)) }
    or(...conds: Query[]): Query { return qAny(...[this as Query].concat(conds)) }
    not(): Query { return qNot(this as Query) }
    toApiQueryExpr(): object {
        const ret = {}
        if (this.__yoQOp === "NOT")
            ret["NOT"] = this.__yoQConds[0].toApiQueryExpr()
        else if ((this.__yoQOp === "AND") || (this.__yoQOp === "OR"))
            ret[this.__yoQOp] = this.__yoQConds.map((_) => _.toApiQueryExpr())
        else
            ret[this.__yoQOp] = this.__yoQOperands.map((_) => _.toApiQueryExpr())
        return ret
    }
}

export class qL<T extends (string | number | boolean | null)>  {
    __yoQLitValue: T
    constructor(val: T) { this.__yoQLitValue = val }
    equal(other: QVal): Query { return qEqual(this, other) }
    notEqual(other: QVal): Query { return qNotEqual(this, other) }
    lessThan(other: QVal): Query { return qLessThan(this, other) }
    lessOrEqual(other: QVal): Query { return qLessOrEqual(this, other) }
    greaterThan(other: QVal): Query { return qGreaterThan(this, other) }
    greaterOrEqual(other: QVal): Query { return qGreaterOrEqual(this, other) }
    in(...set: QVal[]): Query { return qIn(this, ...set) }
    toApiQueryExpr(): object {
        if (typeof this.__yoQLitValue === 'string')
            return { 'Str': this.__yoQLitValue ?? '' }
        if (typeof this.__yoQLitValue === 'number')
            return { 'Int': this.__yoQLitValue ?? 0 }
        if (typeof this.__yoQLitValue === 'boolean')
            return { 'Bool': this.__yoQLitValue ?? '' }
        return null
    }
}

export class qF<T extends string> {
    __yoQFieldName: T
    constructor(fieldName: T) { this.__yoQFieldName = fieldName }
    equal(other: QVal): Query { return qEqual(this, other) }
    notEqual(other: QVal): Query { return qNotEqual(this, other) }
    lessThan(other: QVal): Query { return qLessThan(this, other) }
    lessOrEqual(other: QVal): Query { return qLessOrEqual(this, other) }
    greaterThan(other: QVal): Query { return qGreaterThan(this, other) }
    greaterOrEqual(other: QVal): Query { return qGreaterOrEqual(this, other) }
    in(...set: QVal[]): Query { return qIn(this, ...set) }
    toApiQueryExpr(): object { return { "Fld": this.__yoQFieldName } }
}

function qAll(...conds: Query[]): Query { return { __yoQOp: "AND", __yoQConds: conds } as Query }
function qAny(...conds: Query[]): Query { return { __yoQOp: "OR", __yoQConds: conds } as Query }
function qNot(cond: Query): Query { return { __yoQOp: "NOT", __yoQConds: [cond] } as Query }

function qEqual(x: QVal, y: QVal): Query { return { __yoQOp: "EQ", __yoQOperands: [x, y] } as Query }
function qNotEqual(x: QVal, y: QVal): Query { return { __yoQOp: "NE", __yoQOperands: [x, y] } as Query }
function qLessThan(x: QVal, y: QVal): Query { return { __yoQOp: "LT", __yoQOperands: [x, y] } as Query }
function qLessOrEqual(x: QVal, y: QVal): Query { return { __yoQOp: "LE", __yoQOperands: [x, y] } as Query }
function qGreaterThan(x: QVal, y: QVal): Query { return { __yoQOp: "GT", __yoQOperands: [x, y] } as Query }
function qGreaterOrEqual(x: QVal, y: QVal): Query { return { __yoQOp: "GE", __yoQOperands: [x, y] } as Query }
function qIn(x: QVal, ...set: QVal[]): Query { return { __yoQOp: "IN", __yoQOperands: [x].concat(set) } as Query }
