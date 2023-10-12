package yofeat_auth

import q "yo/db/query"

type UserAccountCol = q.C

const (
	UserAccountColId             = UserAccountCol("id_")
	UserAccountColCreated        = UserAccountCol("created_")
	UserAccountColEmailAddr      = UserAccountCol("email_addr_")
	UserAccountColPasswordHashed = UserAccountCol("password_hashed_")
)

type UserAccountField q.F

const (
	UserAccountFieldId             UserAccountField = "Id"
	UserAccountFieldCreated        UserAccountField = "Created"
	UserAccountFieldEmailAddr      UserAccountField = "EmailAddr"
	userAccountFieldPasswordHashed UserAccountField = "passwordHashed"
)

func (me UserAccountField) Asc() q.OrderBy                { return ((q.F)(me)).Asc() }
func (me UserAccountField) Desc() q.OrderBy               { return ((q.F)(me)).Desc() }
func (me UserAccountField) In(set ...any) q.Query         { return ((q.F)(me)).In(set...) }
func (me UserAccountField) NotIn(set ...any) q.Query      { return ((q.F)(me)).NotIn(set...) }
func (me UserAccountField) Equal(other any) q.Query       { return ((q.F)(me)).Equal(other) }
func (me UserAccountField) NotEqual(other any) q.Query    { return ((q.F)(me)).NotEqual(other) }
func (me UserAccountField) LessThan(other any) q.Query    { return ((q.F)(me)).LessThan(other) }
func (me UserAccountField) GreaterThan(other any) q.Query { return ((q.F)(me)).GreaterThan(other) }
func (me UserAccountField) LessOrEqual(other any) q.Query { return ((q.F)(me)).LessOrEqual(other) }
func (me UserAccountField) GreaterOrEqual(other any) q.Query {
	return ((q.F)(me)).GreaterOrEqual(other)
}
