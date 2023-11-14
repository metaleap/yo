package yosrv_test

import (
	"reflect"
	"testing"

	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

type None struct{}

func Test_Repro2(t *testing.T) {
	oarefl := openapi31.NewReflector()
	oarefl.Spec.Info.WithTitle("kaffe.local")
	oarefl.JSONSchemaReflector().DefaultOptions = append(oarefl.JSONSchemaReflector().DefaultOptions, jsonschema.ProcessWithoutTags)

	{
		var dummy_in struct{ Id yodb_I64 }
		var dummy_out None
		ty_args, ty_ret := reflect.TypeOf(dummy_in), reflect.TypeOf(dummy_out)
		op, err := oarefl.NewOperationContext("POST", "/_/postDelete")
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
