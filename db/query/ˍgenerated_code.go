package q

func (me C) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me F) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me V) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
