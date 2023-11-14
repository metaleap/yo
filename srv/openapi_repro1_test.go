package yosrv_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

type yodb_I64 int64
type yodb_DateTime time.Time
type Return[T any] struct{ Result T }

type Post struct {
	Id     yodb_I64       `json:"Id"`
	DtMade *yodb_DateTime `json:"DtMade"`
}

func Test_Repro1(t *testing.T) {
	oarefl := openapi31.NewReflector()
	oarefl.Spec.Info.WithTitle("kaffe.local")
	oarefl.JSONSchemaReflector().DefaultOptions = append(oarefl.JSONSchemaReflector().DefaultOptions, jsonschema.ProcessWithoutTags)

	{
		var dummy_in Post
		var dummy_out Return[yodb_I64]
		ty_args, ty_ret := reflect.TypeOf(dummy_in), reflect.TypeOf(dummy_out)
		op, err := oarefl.NewOperationContext("POST", "/_/postNew")
		if err != nil {
			t.Fatal(err)
		}
		op.AddReqStructure(reflect.New(ty_args).Elem().Interface(), openapi.WithContentType("application/json"))
		op.AddRespStructure(reflect.New(ty_ret).Elem().Interface(), openapi.WithHTTPStatus(200))
		if err = oarefl.AddOperation(op); err != nil {
			t.Fatal(err)
		}
	}

	src_json, err := oarefl.Spec.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(src_json))
}

func (me *yodb_DateTime) UnmarshalJSON(data []byte) error {
	return ((*time.Time)(me)).UnmarshalJSON(data)
}
func (me *yodb_DateTime) MarshalJSON() ([]byte, error) { return ((*time.Time)(me)).MarshalJSON() }
