package q

func (me C) StrLen(args ...Operand) Operand { return Fn(FnStrLen, append([]Operand{me}, args...)...) }
func (me F) StrLen(args ...Operand) Operand { return Fn(FnStrLen, append([]Operand{me}, args...)...) }
func (me V[T]) StrLen(args ...Operand) Operand {
	return Fn(FnStrLen, append([]Operand{me}, args...)...)
}
