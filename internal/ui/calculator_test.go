package ui

import (
	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"os"
	"path/filepath"
	"testing"
)

func calculatorWidgets(u *UI) (*tview.Form, *tview.TextView, *tview.TextView) {
	_, page := u.pages.GetFrontPage()
	layout := page.(*tview.Flex).GetItem(1).(*tview.Flex).GetItem(1).(*tview.Flex)
	return layout.GetItem(0).(*tview.Form), layout.GetItem(1).(*tview.TextView), layout.GetItem(2).(*tview.TextView)
}

func TestCalculatorRestoreUnitsAndCopy(t *testing.T) {
	clipboard := mockClipboard(t)
	u := editorFixture(t)
	u.calculator()
	f, final, details := calculatorWidgets(u)
	if idx, _ := unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).GetCurrentOption(); idx != 1 {
		t.Fatal("first token not selected", idx)
	}
	unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).SetCurrentOption(2)
	unwrapFormItem(f.GetFormItem(4)).(*tview.InputField).SetText("1")
	unwrapFormItem(f.GetFormItem(5)).(*tview.DropDown).SetCurrentOption(1)
	pressButton(f, "계산")
	if got := final.GetText(false); got != "1000000000000000000axpla" {
		t.Fatal(got)
	}
	pressButton(f, "결과 복사")
	data, err := os.ReadFile(clipboard)
	if err != nil || string(data) != final.GetText(false) {
		t.Fatal(string(data), err)
	}
	previous := details.GetText(false)
	u.close()
	// A new UI instance proves restoration does not rely on modal memory.
	v := editorFixture(t)
	v.dir = u.dir
	v.calculator()
	f, final, details = calculatorWidgets(v)
	if final.GetText(false) != "1000000000000000000axpla" || details.GetText(false) != previous {
		t.Fatal("result reset on reopen")
	}
	if unwrapFormItem(f.GetFormItem(4)).(*tview.InputField).GetText() != "1" {
		t.Fatal("expression reset")
	}
	if idx, _ := unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).GetCurrentOption(); idx != 2 {
		t.Fatal("token reset")
	}
	unwrapFormItem(f.GetFormItem(4)).(*tview.InputField).SetText("1000000000000000000")
	unwrapFormItem(f.GetFormItem(5)).(*tview.DropDown).SetCurrentOption(2)
	pressButton(f, "계산")
	if final.GetText(false) != "1 XPLA" {
		t.Fatal(final.GetText(false))
	}
	pressButton(f, "결과 복사")
	data, err = os.ReadFile(clipboard)
	if err != nil || string(data) != "1 XPLA" {
		t.Fatal(string(data), err)
	}
	unwrapFormItem(f.GetFormItem(4)).(*tview.InputField).SetText("(")
	pressButton(f, "계산")
	if final.GetText(false) != "" {
		t.Fatal("stale transferable result after error")
	}
}

func TestCalculatorDefaultSkipsIBC(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tokens []garden.Token
		saved  bool
		want   int
	}{
		{"skip IBC", []garden.Token{{Symbol: "IBC", Denom: "ibc/ABC", Decimals: 6}, {Symbol: "XPLA", Denom: "axpla", Decimals: 18}}, false, 2},
		{"only IBC", []garden.Token{{Symbol: "IBC", Denom: "ibc/ABC", Decimals: 6}}, false, 0},
		{"restore IBC choice", []garden.Token{{Symbol: "IBC", Denom: "ibc/ABC", Decimals: 6}, {Symbol: "XPLA", Denom: "axpla", Decimals: 18}}, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := editorFixture(t)
			if err := garden.Save(filepath.Join(u.dir, "tokens.json"), tc.tokens); err != nil {
				t.Fatal(err)
			}
			if tc.saved {
				if err := garden.Save(filepath.Join(u.dir, "calculator.json"), calculatorState{Token: "ibc/ABC", Symbol: "IBC", Denom: "ibc/ABC", Decimal: "6"}); err != nil {
					t.Fatal(err)
				}
			}
			u.calculator()
			f, _, _ := calculatorWidgets(u)
			if got, _ := unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).GetCurrentOption(); got != tc.want {
				t.Fatalf("selection = %d, want %d", got, tc.want)
			}
			wantDenom := ""
			if tc.want > 0 {
				wantDenom = tc.tokens[tc.want-1].Denom
			}
			if got := unwrapFormItem(f.GetFormItem(2)).(*tview.InputField).GetText(); got != wantDenom {
				t.Fatalf("denom = %q, want %q", got, wantDenom)
			}
		})
	}
}

func TestCalculatorF12Toggle(t *testing.T) {
	u := editorFixture(t)
	u.installInput()
	press := func() { u.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyF12, 0, tcell.ModNone)) }
	press()
	f, final, _ := calculatorWidgets(u)
	unwrapFormItem(f.GetFormItem(4)).(*tview.InputField).SetText("12 + 34")
	pressButton(f, "계산")
	want := final.GetText(false)
	bar := &functionBar{ui: u}
	if bar.labels()[10] != "닫기" {
		t.Fatal("missing close hint")
	}
	press()
	if name, _ := u.pages.GetFrontPage(); name != "command" {
		t.Fatal("calculator did not close", name)
	}
	if !u.editor.form.HasFocus() {
		t.Fatal("command form focus not restored")
	}
	press()
	f, final, _ = calculatorWidgets(u)
	if unwrapFormItem(f.GetFormItem(4)).(*tview.InputField).GetText() != "12 + 34" || final.GetText(false) != want {
		t.Fatal("calculator state reset")
	}
	u.close()
	u.settings()
	press()
	if _, page := u.pages.GetFrontPage(); page != u.calculatorPage || page == nil {
		t.Fatal("F12 closed another dialog instead of opening calculator")
	}
}
