package garden

import (
	"reflect"
	"testing"
)

func TestFieldsOptionalByDefault(t *testing.T) {
	c := &Command{Usage: "demo send <sender> [recipient] [coins] [flags]", Flags: []Flag{{Name: "node", Type: "string"}, {Name: "from", Type: "string", Description: "signer (required)"}, {Name: "output", Type: "string", Description: "format (text|json)"}}}
	fields := Fields(c)
	if len(fields) != 6 || !fields[0].Required || fields[1].Required || fields[3].Required || !fields[4].Required {
		t.Fatalf("%+v", fields)
	}
	if !reflect.DeepEqual(fields[5].Choices, []string{"text", "json"}) {
		t.Fatal(fields[5])
	}
}
func TestFormArgsValidation(t *testing.T) {
	c := &Command{Path: []string{"query", "block"}, Usage: "demo query block <height> [label] [flags]", Flags: []Flag{{Name: "trace", Type: "bool"}, {Name: "node", Type: "string"}}}
	fields := Fields(c)
	values := map[string]FieldValue{}
	if _, e := FormArgs(c, fields, values); e == nil {
		t.Fatal("missing required value accepted")
	}
	values["arg:0"] = FieldValue{Enabled: true, Value: "-1"}
	if _, e := FormArgs(c, fields, values); e == nil {
		t.Fatal("invalid uint accepted")
	}
	values["arg:0"] = FieldValue{Enabled: true, Value: "42"}
	values["arg:1"] = FieldValue{Enabled: true, Value: "--bad=$(touch injected)"}
	values["flag:trace"] = FieldValue{Enabled: true, Value: "false"}
	values["flag:node"] = FieldValue{Enabled: false, Value: "invalid disabled"}
	args, e := FormArgs(c, fields, values)
	want := []string{"query", "block", "--trace=false", "--", "42", "--bad=$(touch injected)"}
	if e != nil || !reflect.DeepEqual(args, want) {
		t.Fatal(args, e)
	}
}
func TestOptionalGap(t *testing.T) {
	c := &Command{Usage: "demo [first] [second]"}
	fields := Fields(c)
	if _, e := FormArgs(c, fields, map[string]FieldValue{"arg:1": {Enabled: true, Value: "second"}}); e == nil {
		t.Fatal("position shifted silently")
	}
	if _, e := FormArgs(c, fields, nil); e != nil {
		t.Fatal("uncertain args must be optional", e)
	}
}
func TestFieldTypes(t *testing.T) {
	for _, tc := range []struct {
		field Field
		value string
		valid bool
	}{{Field{Type: "uint8"}, "256", false}, {Field{Type: "uint64"}, "18446744073709551615", true}, {Field{Type: "int8"}, "-128", true}, {Field{Type: "duration"}, "2m", true}, {Field{Type: "duration"}, "abc", false}, {Field{Type: "float64"}, "NaN", false}, {Field{Type: "address"}, "garbage", false}, {Field{Type: "coins"}, "2.5uatom,1axpla", true}, {Field{Flag: true, Name: "gas", Type: "string"}, "auto", true}, {Field{Flag: true, Name: "node", Type: "string"}, "tcp://localhost:26657", true}, {Field{Flag: true, Name: "node", Type: "string"}, "localhost", false}} {
		e := ValidateField(tc.field, tc.value)
		if (e == nil) != tc.valid {
			t.Errorf("%+v %q: %v", tc.field, tc.value, e)
		}
	}
}
func TestCommonFlagsExcludeSecrets(t *testing.T) {
	if CommonFlag("password") || CommonFlag("mnemonic") || CommonFlag("yes") || !CommonFlag("chain-id") {
		t.Fatal("incorrect persistence allowlist")
	}
}
