package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"garden_cosmos/internal/garden"
	"github.com/rivo/tview"
)

type calculatorState struct {
	Token                              string
	Symbol, Denom, Decimal, Expression string
	Mode                               int
	Result, Final                      string
}

func (u *UI) calculator() {
	if err := u.flushCalculator(); err != nil {
		u.alert(err.Error())
		return
	}
	presets := []garden.Token{{Symbol: "ATOM", Denom: "uatom", Decimals: 6}, {Symbol: "XPLA", Denom: "axpla", Decimals: 18}}
	if e := garden.Load(filepath.Join(u.dir, "tokens.json"), &presets); e != nil && !os.IsNotExist(e) {
		u.alert(e.Error())
		return
	}
	state := calculatorState{Decimal: "18", Expression: "0.1 + 0.2"}
	statePath := filepath.Join(u.dir, "calculator.json")
	err := garden.Load(statePath, &state)
	if err != nil && !os.IsNotExist(err) {
		u.alert(err.Error())
		return
	}
	if os.IsNotExist(err) {
		for _, p := range presets {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.Denom)), "ibc/") {
				continue
			}
			state.Token, state.Symbol, state.Denom, state.Decimal = p.Denom, p.Symbol, p.Denom, strconv.Itoa(p.Decimals)
			break
		}
	}
	if state.Mode < 0 || state.Mode > 2 {
		state.Mode = 0
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
	field := func(i int) *tview.InputField { return unwrapFormItem(f.GetFormItem(i)).(*tview.InputField) }
	unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).SetSelectedFunc(func(_ string, i int) {
		if i > 0 {
			p := presets[i-1]
			field(1).SetText(p.Symbol)
			field(2).SetText(p.Denom)
			field(3).SetText(strconv.Itoa(p.Decimals))
		}
	})
	tokenIndex := 0
	for i, p := range presets {
		if p.Denom == state.Token {
			tokenIndex = i + 1
			break
		}
	}
	unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).SetCurrentOption(tokenIndex)
	for i, value := range []string{state.Symbol, state.Denom, state.Decimal, state.Expression} {
		field(i + 1).SetText(value)
	}
	unwrapFormItem(f.GetFormItem(5)).(*tview.DropDown).SetCurrentOption(state.Mode)
	result.SetText(state.Result)
	final := tview.NewTextView().SetWrap(true).SetText(state.Final)
	final.SetBorder(true).SetTitle(" 최종 변환 결과 ")
	saver := &deferredSave{path: statePath, delay: 300 * time.Millisecond}
	u.calculatorSave = saver
	save := func() {
		i, _ := unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).GetCurrentOption()
		state.Token = ""
		if i > 0 {
			state.Token = presets[i-1].Denom
		}
		state.Symbol, state.Denom, state.Decimal, state.Expression = field(1).GetText(), field(2).GetText(), field(3).GetText(), field(4).GetText()
		state.Mode, _ = unwrapFormItem(f.GetFormItem(5)).(*tview.DropDown).GetCurrentOption()
		state.Result, state.Final = result.GetText(false), final.GetText(false)
		saver.Schedule(state)
	}
	var generation uint64
	changed := func() {
		generation++
		if u.calculatorCancel != nil {
			u.calculatorCancel()
		}
		save()
	}
	for i := 1; i <= 4; i++ {
		field(i).SetChangedFunc(func(string) { changed() })
	}
	unwrapFormItem(f.GetFormItem(5)).(*tview.DropDown).SetSelectedFunc(func(string, int) { changed() })
	unwrapFormItem(f.GetFormItem(0)).(*tview.DropDown).SetSelectedFunc(func(_ string, i int) {
		if i > 0 {
			p := presets[i-1]
			field(1).SetText(p.Symbol)
			field(2).SetText(p.Denom)
			field(3).SetText(strconv.Itoa(p.Decimals))
		}
		save()
	})
	calculate := func(insert bool) {
		if u.calculatorCancel != nil {
			u.calculatorCancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		u.calculatorCancel = cancel
		generation++
		job := generation
		expression, decimal, symbol, denom := field(4).GetText(), field(3).GetText(), field(1).GetText(), field(2).GetText()
		mode, modeLabel := unwrapFormItem(f.GetFormItem(5)).(*tview.DropDown).GetCurrentOption()
		_, origin := u.pages.GetFrontPage()
		final.SetText("")
		result.SetText("계산 중…")
		save()
		work := func() {
			details, formatted, transferable, err := calculateToken(ctx, expression, decimal, symbol, denom, mode, modeLabel)
			apply := func() {
				_, page := u.pages.GetFrontPage()
				if ctx.Err() != nil || job != generation || page != origin {
					return
				}
				if err != nil {
					result.SetText("[red]" + tview.Escape(err.Error()))
				} else {
					result.SetText(tview.Escape(details))
					final.SetText(formatted)
				}
				save()
				if insert && transferable && err == nil {
					if u.appendArg(formatted) {
						u.close()
					} else {
						result.SetText("명령 입력 폼에서 사용할 입력 칸을 활성화하고 선택한 뒤 계산기를 여세요.")
					}
				}
			}
			if ctx.Err() != nil {
				return
			}
			if u.postTask != nil {
				u.postTask(apply)
			} else {
				apply()
			}
		}
		if u.launchTask != nil {
			u.launchTask(work)
		} else {
			work()
		}
	}
	f.AddButton("계산", func() { calculate(false) }).AddButton("인자에 삽입", func() { calculate(true) })
	if u.hasClipboard() {
		f.AddButton("결과 복사", func() {
			if value := final.GetText(false); value != "" {
				if err := u.copyClipboard(value); err != nil {
					result.SetText(tview.Escape(err.Error()))
					save()
				}
			}
		})
	}
	f.AddButton("토큰 저장", func() {
		defer save()
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
	f.SetBorder(true).SetTitle(" F12 계산기 · + − * / ( ) · 과학적 표기 1e18 · F12/Esc 닫기 ")
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(f, 19, 0, true).AddItem(final, 3, 0, false).AddItem(result, 0, 1, false)
	u.modal(layout, 96, 31)
	_, u.calculatorPage = u.pages.GetFrontPage()
}

// Pure bounded computation: UI workers capture values, never read live widgets.
func calculateToken(ctx context.Context, expression, decimal, symbol, denom string, mode int, modeLabel string) (string, string, bool, error) {
	v, err := garden.CalculateContext(ctx, expression)
	if err != nil {
		return "", "", false, err
	}
	text := "수식 값: " + garden.FormatRat(v)
	if mode > 0 {
		d, err := strconv.Atoi(decimal)
		if err != nil {
			return "", "", false, fmt.Errorf("decimal은 정수로 입력하세요")
		}
		v, err = garden.Scale(v, d, mode == 1)
		if err != nil {
			return "", "", false, err
		}
		text += "\n" + modeLabel + ": " + garden.FormatRat(v)
		if mode == 1 && !v.IsInt() {
			return text + "\n최소 단위가 정수가 아닙니다. 반올림하지 않았으며 인자에 삽입할 수 없습니다.", "", false, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return "", "", false, err
	}
	numeric := garden.FormatRat(v)
	transferable := !strings.Contains(numeric, "/")
	formatted := numeric
	if mode == 1 {
		formatted += denom
	} else if mode == 2 {
		formatted += " " + symbol
	}
	return text, formatted, transferable, nil
}
