package garden

import (
	"reflect"
	"testing"
)

func TestPositionalFieldsRequired(t *testing.T) {
	c := &Command{Usage: "demo send <sender> [recipient] [coins] [flags]", Flags: []Flag{{Name: "node", Type: "string"}, {Name: "from", Type: "string", Description: "signer (required)"}, {Name: "output", Type: "string", Description: "format (text|json)"}}}
	fields := Fields(c)
	if len(fields) != 6 || !fields[0].Required || !fields[1].Required || !fields[2].Required || !fields[3].Required || fields[4].Required {
		t.Fatalf("%+v", fields)
	}
	if fields[3].Name != "from" || fields[4].Name != "node" {
		t.Fatal("required flag not first", fields)
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
	if _, e := FormArgs(c, fields, nil); e == nil {
		t.Fatal("bracketed args must be required")
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
	if CommonFlag("password") || CommonFlag("mnemonic") || !CommonFlag("yes") || !CommonFlag("limit") || CommonFlag("seed") || !CommonFlag("chain-id") {
		t.Fatal("incorrect persistence allowlist")
	}
}

func TestBankSendBracketedArguments(t *testing.T) {
	c := &Command{Path: []string{"tx", "bank", "send"}, Usage: "xplad tx bank send [from_key_or_address] [to_address] [amount] [flags]"}
	fields := Fields(c)
	if len(fields) != 3 {
		t.Fatalf("fields: %+v", fields)
	}
	values := map[string]FieldValue{}
	for i, f := range fields {
		if !f.Required {
			t.Fatal("optional positional", f.Name)
		}
		values[f.Key] = FieldValue{Value: []string{"alice", "recipient", "1axpla"}[i]}
	}
	for _, f := range fields {
		saved := values[f.Key]
		values[f.Key] = FieldValue{}
		if _, err := FormArgs(c, fields, values); err == nil {
			t.Fatal("missing argument accepted", f.Name)
		}
		values[f.Key] = saved
	}
	if _, err := FormArgs(c, fields, values); err != nil {
		t.Fatal(err)
	}
}

func TestUsageFlagAlternativesAreNotArguments(t *testing.T) {
	for _, usage := range []string{"xplad query block --type=[height|hash] [height|hash] [flags]", "xplad query block --type <height|hash> [height|hash] [flags]"} {
		c := &Command{Path: []string{"query", "block"}, Usage: usage, Flags: []Flag{{Name: "height", Type: "int"}, {Name: "type", Type: "string", Default: "hash"}}}
		fields := Fields(c)
		if len(fields) != 3 || fields[0].Flag || fields[0].Name != "height|hash" || !fields[0].Required {
			t.Fatalf("duplicate positional fields: %+v", fields)
		}
		if !fields[1].Flag || !fields[1].Required || !reflect.DeepEqual(fields[1].Choices, []string{"height", "hash"}) {
			t.Fatalf("flag choices: %+v", fields[1])
		}
		if fields[2].Required {
			t.Fatal("unmentioned flag became required")
		}
		values := map[string]FieldValue{"arg:0": {Enabled: true, Value: "123"}, "flag:type": {Enabled: true, Value: "height"}}
		argv, err := FormArgs(c, fields, values)
		if err != nil || !reflect.DeepEqual(argv, []string{"query", "block", "--type=height", "123"}) {
			t.Fatal(argv, err)
		}
		values["flag:type"] = FieldValue{Value: "wrong"}
		if _, err := FormArgs(c, fields, values); err == nil {
			t.Fatal("invalid required flag accepted")
		}
	}
	fields := Fields(&Command{Usage: "demo [--type=[height|hash]] [value] [flags]", Flags: []Flag{{Name: "type", Type: "string"}}})
	if len(fields) != 2 || fields[1].Required {
		t.Fatalf("optional flag wrapper: %+v", fields)
	}
}

func TestCommunityPoolUsageOrder(t *testing.T) {
	c := &Command{Path: []string{"tx", "distribution", "fund-community-pool"}, Usage: "xplad tx distribution fund-community-pool [amount] [flags]", Flags: []Flag{{Name: "gas-adjustment", Type: "string"}, {Name: "gas-prices", Type: "string"}}}
	fields := Fields(c)
	values := map[string]FieldValue{"arg:0": {Value: "100axpla"}, "flag:gas-adjustment": {Enabled: true, Value: "1.1"}, "flag:gas-prices": {Enabled: true, Value: "1200000000000axpla"}}
	want := []string{"tx", "distribution", "fund-community-pool", "100axpla", "--gas-adjustment=1.1", "--gas-prices=1200000000000axpla"}
	got, err := FormArgs(c, fields, values)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	if preview := CommandArgs(c, fields, values); !reflect.DeepEqual(preview, got) {
		t.Fatal("preview differs", preview)
	}
	values["arg:0"] = FieldValue{}
	want[3] = ""
	if preview := CommandArgs(c, fields, values); !reflect.DeepEqual(preview, want) {
		t.Fatal("empty preview position", preview)
	}
}
