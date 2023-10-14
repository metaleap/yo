package q

import (
	"fmt"
	"testing"
)

type Obj struct {
	Num int8
	Str string
}

func TestEval(t *testing.T) {
	obj1 := Obj{Str: "foo", Num: 42}
	q1 := L(456).LessOrEqual(L(321)).Or(L("a").LessThan(L("z")))
	q2 := L("foo").Equal(F("Str"))
	q3 := Fn(FnStrLen, L("barbaz")).Equal(L(6))
	println("Q1", fmt.Sprintf("%#v", nil == q1.Eval(nil, nil)))
	println("Q2", fmt.Sprintf("%#v", nil == q2.Eval(&obj1, nil)))
	println("Q3", fmt.Sprintf("%#v", nil == q3.Eval(&obj1, nil)))
}
