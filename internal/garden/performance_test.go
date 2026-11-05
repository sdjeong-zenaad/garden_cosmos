package garden

import (
	"math/big"
	"testing"
)

func BenchmarkFormatRat(b *testing.B) {
	value, _ := new(big.Rat).SetString("1e-1000")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		FormatRat(value)
	}
}

func BenchmarkFlatten(b *testing.B) {
	root := &Command{}
	for i := 0; i < 100; i++ {
		group := &Command{}
		for j := 0; j < 10; j++ {
			group.Children = append(group.Children, &Command{})
		}
		root.Children = append(root.Children, group)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Flatten(root)
	}
}

func TestFormatRatPreservesExactValue(t *testing.T) {
	for _, fraction := range []string{"1/2", "1/5", "-7/40", "3/125", "1e-1000", "123.4500"} {
		value, _ := new(big.Rat).SetString(fraction)
		before := new(big.Rat).Set(value)
		rendered := FormatRat(value)
		parsed, ok := new(big.Rat).SetString(rendered)
		if !ok || parsed.Cmp(before) != 0 || value.Cmp(before) != 0 {
			t.Fatalf("%s: incorrect formatting or mutated input: %s", fraction, rendered)
		}
	}
	value := big.NewRat(-1, 6)
	if got := FormatRat(value); got != "-1/6  (≈ -0.166666666666666667)" {
		t.Fatal(got)
	}
}

func TestFlattenOrder(t *testing.T) {
	leaf := &Command{Summary: "leaf"}
	group := &Command{Summary: "group", Children: []*Command{leaf}}
	sibling := &Command{Summary: "sibling"}
	root := &Command{Children: []*Command{group, sibling}}
	got := Flatten(root)
	want := []*Command{root, group, leaf, sibling}
	if len(got) != len(want) {
		t.Fatal(len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order changed at %d", i)
		}
	}
	if got := Flatten(nil); len(got) != 0 {
		t.Fatal(got)
	}
}
