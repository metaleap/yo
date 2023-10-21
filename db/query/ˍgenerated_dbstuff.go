package q

func (me C) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me F) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me V) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me C) ArrLen(args ...any) Operand { return Fn(FnArrLen, append([]any{me}, args...)...) }
func (me F) ArrLen(args ...any) Operand { return Fn(FnArrLen, append([]any{me}, args...)...) }
func (me V) ArrLen(args ...any) Operand { return Fn(FnArrLen, append([]any{me}, args...)...) }
