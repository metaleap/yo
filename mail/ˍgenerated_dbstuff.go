// Code generated by `yo/db/codegen_dbstuff.go`. DO NOT EDIT
package yomail

import q "yo/db/query"

import sl "yo/util/sl"

func MailReqFields(fields ...MailReqField) []q.F { return sl.To(fields, MailReqField.F) }

type MailReqField q.F

const (
	MailReqId       MailReqField = "Id"
	MailReqDtMade   MailReqField = "DtMade"
	MailReqDtMod    MailReqField = "DtMod"
	MailReqTmplId   MailReqField = "TmplId"
	MailReqTmplArgs MailReqField = "TmplArgs"
	MailReqMailTo   MailReqField = "MailTo"
	MailReqMailCc   MailReqField = "MailCc"
	MailReqMailBcc  MailReqField = "MailBcc"
	mailReqDtDone   MailReqField = "dtDone"
)

func (me MailReqField) ArrLen(a1 ...interface{}) q.Operand { return ((q.F)(me)).ArrLen(a1...) }
func (me MailReqField) Asc() q.OrderBy                     { return ((q.F)(me)).Asc() }
func (me MailReqField) Desc() q.OrderBy                    { return ((q.F)(me)).Desc() }
func (me MailReqField) Equal(a1 interface{}) q.Query       { return ((q.F)(me)).Equal(a1) }
func (me MailReqField) Eval(a1 interface{}, a2 func(q.C) q.F) interface{} {
	return ((q.F)(me)).Eval(a1, a2)
}
func (me MailReqField) F() q.F                                { return ((q.F)(me)).F() }
func (me MailReqField) GreaterOrEqual(a1 interface{}) q.Query { return ((q.F)(me)).GreaterOrEqual(a1) }
func (me MailReqField) GreaterThan(a1 interface{}) q.Query    { return ((q.F)(me)).GreaterThan(a1) }
func (me MailReqField) In(a1 ...interface{}) q.Query          { return ((q.F)(me)).In(a1...) }
func (me MailReqField) InArr(a1 interface{}) q.Query          { return ((q.F)(me)).InArr(a1) }
func (me MailReqField) LessOrEqual(a1 interface{}) q.Query    { return ((q.F)(me)).LessOrEqual(a1) }
func (me MailReqField) LessThan(a1 interface{}) q.Query       { return ((q.F)(me)).LessThan(a1) }
func (me MailReqField) Not() q.Query                          { return ((q.F)(me)).Not() }
func (me MailReqField) NotEqual(a1 interface{}) q.Query       { return ((q.F)(me)).NotEqual(a1) }
func (me MailReqField) NotIn(a1 ...interface{}) q.Query       { return ((q.F)(me)).NotIn(a1...) }
func (me MailReqField) NotInArr(a1 interface{}) q.Query       { return ((q.F)(me)).NotInArr(a1) }
func (me MailReqField) StrLen(a1 ...interface{}) q.Operand    { return ((q.F)(me)).StrLen(a1...) }
