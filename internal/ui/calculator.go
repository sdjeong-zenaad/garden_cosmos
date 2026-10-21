package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"garden_cosmos/internal/garden"
	"github.com/rivo/tview"
)

func (u *UI) calculator() {
	presets := []garden.Token{{Symbol: "ATOM", Denom: "uatom", Decimals: 6}, {Symbol: "XPLA", Denom: "axpla", Decimals: 18}}
	if e := garden.Load(filepath.Join(u.dir, "tokens.json"), &presets); e != nil && !os.IsNotExist(e) {
		u.alert(e.Error())
		return
	}
	f := tview.NewForm()
	result := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	result.SetBorder(true).SetTitle(" 계산 결과 · 정확한 값 / 순환소수는 분수로 표시 ")
	labels := []string{"직접 입력"}
	for _, p := range presets {
		labels = append(labels, fmt.Sprintf("%s / %s (%d)", p.Symbol, p.Denom, p.Decimals))
	}
	f.AddDropDown("토큰", labels, 0, nil).AddInputField("심볼", "", 34, nil, nil).AddInputField("최소 단위 denom", "", 34, nil, nil).AddInputField("decimal", "18", 8, nil, nil).AddInputField("수식", "0.1 + 0.2", 55, nil, nil)
	f.AddDropDown("변환", []string{"사칙연산만", "표시 단위 → 최소 단위", "최소 단위 → 표시 단위"}, 0, nil)
	field := func(i int) *tview.InputField { return f.GetFormItem(i).(*tview.InputField) }
	f.GetFormItem(0).(*tview.DropDown).SetSelectedFunc(func(_ string, i int) {
		if i > 0 {
			p := presets[i-1]
			field(1).SetText(p.Symbol)
			field(2).SetText(p.Denom)
			field(3).SetText(strconv.Itoa(p.Decimals))
		}
	})
	calculate := func() string {
		v, e := garden.Calculate(field(4).GetText())
		if e != nil {
			result.SetText("[red]" + tview.Escape(e.Error()))
			return ""
		}
		_, mode := f.GetFormItem(5).(*tview.DropDown).GetCurrentOption()
		text := "수식 값: " + garden.FormatRat(v)
		idx, _ := f.GetFormItem(5).(*tview.DropDown).GetCurrentOption()
		if idx > 0 {
			d, e := strconv.Atoi(field(3).GetText())
			if e != nil {
				result.SetText("[red]decimal은 정수로 입력하세요")
				return ""
			}
			v, e = garden.Scale(v, d, idx == 1)
			if e != nil {
				result.SetText("[red]" + tview.Escape(e.Error()))
				return ""
			}
			text += "\n" + mode + ": " + garden.FormatRat(v)
			if idx == 1 && !v.IsInt() {
				result.SetText(tview.Escape(text) + "\n[yellow]최소 단위가 정수가 아닙니다. 반올림하지 않았으며 인자에 삽입할 수 없습니다.")
				return ""
			}
		}
		result.SetText(tview.Escape(text))
		if v.IsInt() {
			return v.Num().String()
		}
		formatted := garden.FormatRat(v)
		for _, c := range formatted {
			if c == '/' {
				return ""
			}
		}
		return formatted
	}
	f.AddButton("계산", func() { calculate() }).AddButton("인자에 삽입", func() {
		if v := calculate(); v != "" {
			if u.appendArg(v) {
				u.close()
			} else {
				result.SetText("명령 입력 폼에서 사용할 입력 칸을 활성화하고 선택한 뒤 계산기를 여세요.")
			}
		}
	}).AddButton("토큰 저장", func() {
		d, e := strconv.Atoi(field(3).GetText())
		if e != nil || d < 0 || d > 36 || field(1).GetText() == "" || field(2).GetText() == "" {
			result.SetText("심볼, denom, decimal(0..36)을 확인하세요.")
			return
		}
		p := garden.Token{Symbol: field(1).GetText(), Denom: field(2).GetText(), Decimals: d}
		found := false
		for i, old := range presets {
			if old.Denom == p.Denom {
				presets[i] = p
				found = true
			}
		}
		if !found {
			presets = append(presets, p)
		}
		if e = garden.Save(filepath.Join(u.dir, "tokens.json"), presets); e != nil {
			result.SetText(tview.Escape(e.Error()))
			return
		}
		result.SetText("토큰을 저장했습니다. 다음에 계산기를 열면 목록에 반영됩니다.")
	}).AddButton("닫기", u.close)
	f.SetBorder(true).SetTitle(" F12 계산기 · + − * / ( ) · 과학적 표기 1e18 · Esc 닫기 ")
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(f, 19, 0, true).AddItem(result, 0, 1, false)
	u.modal(layout, 96, 28)
}
