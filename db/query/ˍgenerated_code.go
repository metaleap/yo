package q

func (me C) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me F) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me V) StrLen(args ...any) Operand { return Fn(FnStrLen, append([]any{me}, args...)...) }
func (me C) ArrLen(args ...any) Operand { return Fn(FnArrLen, append([]any{me}, args...)...) }
func (me F) ArrLen(args ...any) Operand { return Fn(FnArrLen, append([]any{me}, args...)...) }
func (me V) ArrLen(args ...any) Operand { return Fn(FnArrLen, append([]any{me}, args...)...) }
func (me C) ArrAll(args ...any) Operand { return Fn(FnArrAll, append([]any{me}, args...)...) }
func (me F) ArrAll(args ...any) Operand { return Fn(FnArrAll, append([]any{me}, args...)...) }
func (me V) ArrAll(args ...any) Operand { return Fn(FnArrAll, append([]any{me}, args...)...) }
func (me C) ArrAny(args ...any) Operand { return Fn(FnArrAny, append([]any{me}, args...)...) }
func (me F) ArrAny(args ...any) Operand { return Fn(FnArrAny, append([]any{me}, args...)...) }
func (me V) ArrAny(args ...any) Operand { return Fn(FnArrAny, append([]any{me}, args...)...) }
