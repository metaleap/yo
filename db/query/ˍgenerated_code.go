package q

func (me C) StrLen(args ...any) Operand { return Fn(FnStrLen, args...) }
func (me F) StrLen(args ...any) Operand { return Fn(FnStrLen, args...) }
func (me V) StrLen(args ...any) Operand { return Fn(FnStrLen, args...) }
