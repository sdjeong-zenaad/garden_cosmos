package garden

import "testing"

func TestCalculatorExact(t *testing.T) {
	for _, tc := range []struct{ expr, want string }{{"0.1 + 0.2", "0.3"}, {"(2 + 3) * 4 / 8", "2.5"}, {"1000000000000000001 - 1000000000000000000", "1"}, {"1e18 + 1", "1000000000000000001"}, {"-.5 * 2", "-1"}} {
		r, e := Calculate(tc.expr)
		if e != nil {
			t.Fatal(e)
		}
		if got := FormatRat(r); got != tc.want {
			t.Errorf("%s: %s != %s", tc.expr, got, tc.want)
		}
	}
}
func TestCalculatorUnits(t *testing.T) {
	r, _ := Calculate("1.000000000000000001")
	base, e := Scale(r, 18, true)
	if e != nil || FormatRat(base) != "1000000000000000001" {
		t.Fatal(base, e)
	}
	back, _ := Scale(base, 18, false)
	if back.Cmp(r) != 0 {
		t.Fatal(back)
	}
	r, _ = Calculate("0.0000001")
	base, _ = Scale(r, 6, true)
	if base.IsInt() {
		t.Fatal("fractional base silently rounded")
	}
	if _, e = Scale(r, 37, true); e == nil {
		t.Fatal("decimal bounds")
	}
}
func TestCalculatorErrors(t *testing.T) {
	for _, s := range []string{"", "1/0", "1 +", "(1+2", "1..2", "exec(1)", "1e1001", "1 2", "1**2"} {
		if _, e := Calculate(s); e == nil {
			t.Error("accepted", s)
		}
	}
	r, _ := Calculate("1/3")
	if got := FormatRat(r); got != "1/3  (≈ 0.333333333333333333)" {
		t.Fatal(got)
	}
}
