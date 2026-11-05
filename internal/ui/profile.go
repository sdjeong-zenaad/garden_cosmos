package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"garden_cosmos/internal/garden"

	"github.com/rivo/tview"
)

func (u *UI) settings() {
	if u.busy {
		u.message("작업 중에는 CLI 설정을 바꿀 수 없습니다.")
		return
	}
	form := tview.NewForm()
	form.AddInputField("CLI 경로", u.profile.Binary, 54, nil, nil).AddInputField("체인 home (선택)", u.profile.Home, 54, nil, nil)
	field := unwrapFormItem(form.GetFormItem(0)).(*tview.InputField)
	styleAutocomplete(field)
	field.SetAutocompleteFunc(func(s string) []string {
		if s == "" {
			return nil
		}
		matches, _ := filepath.Glob(s + "*")
		if len(matches) > 30 {
			matches = matches[:30]
		}
		return matches
	})
	form.AddButton("저장 및 분석", func() {
		p := garden.Profile{Binary: strings.TrimSpace(field.GetText()), Home: strings.TrimSpace(unwrapFormItem(form.GetFormItem(1)).(*tview.InputField).GetText())}
		binary, _, e := garden.Identity(p.Binary)
		if e != nil {
			u.alert(e.Error())
			return
		}
		p.Binary = binary
		if p.Home != "" {
			p.Home, e = filepath.Abs(p.Home)
			if e != nil {
				u.alert(e.Error())
				return
			}
		}
		if err := u.applyProfile(p); err != nil {
			u.alert(err.Error())
			return
		}
		u.refresh(false)
	})
	form.AddButton("현재 CLI 재분석", func() {
		if u.profile.Binary == "" {
			return
		}
		if u.editor != nil {
			u.leaveCommand()
			if u.editor != nil {
				return
			}
		}
		u.close()
		u.refresh(true)
	})
	form.AddButton("취소", u.close).SetBorder(true).SetTitle(" CLI 설정 · Esc 닫기 ")
	u.modal(form, 84, 12)
}
func (u *UI) refresh(force bool) {
	if u.busy {
		return
	}
	if u.profile.Binary == "" {
		u.settings()
		return
	}
	u.busy = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	u.cancel = cancel
	u.message("CLI 도움말 분석 중… Ctrl+C 취소")
	profile := u.profile
	go func() {
		d, e := garden.Refresh(ctx, u.dir, profile, force, func(s string) { u.app.QueueUpdateDraw(func() { u.message("분석 중: " + s + " · Ctrl+C 취소") }) })
		cancel()
		u.app.QueueUpdateDraw(func() {
			u.busy = false
			u.cancel = nil
			if u.quitRequested {
				u.app.Stop()
				return
			}
			if e != nil {
				u.descriptor = nil
				u.selected = nil
				u.visibleCommands = nil
				u.list.Clear()
				u.alert("분석 실패: " + e.Error())
				return
			}
			u.descriptor = d
			u.populate(u.search.GetText())
			u.message(fmt.Sprintf("%s · %s · %d 명령 · 경고 %d · F6 설정/진단", filepath.Base(d.Binary), d.Version, len(garden.Flatten(d.Root)), len(d.Warnings)))
		})
	}()
}

func (u *UI) configs() {
	if u.descriptor == nil {
		return
	}
	b, _ := json.MarshalIndent(struct {
		Home     string
		Configs  []garden.Config
		Warnings []string
	}{u.descriptor.Home, u.descriptor.Configs, u.descriptor.Warnings}, "", "  ")
	v := tview.NewTextView().SetText(string(b)).SetScrollable(true)
	v.SetBorder(true).SetTitle(" 기존 TOML 설정 / 분석 경고 · Esc 닫기 ")
	u.modal(v, 100, 32)
}

// Commit the profile transition before starting asynchronous analysis. Drafts
// retain the old descriptor identity, and no old editor survives the switch.
func (u *UI) applyProfile(p garden.Profile) error {
	if u.editor != nil {
		u.leaveCommand()
		if u.editor != nil {
			return fmt.Errorf("이전 명령 입력을 저장하지 못했습니다")
		}
	}
	if err := garden.Save(filepath.Join(u.dir, "profile.json"), p); err != nil {
		return err
	}
	u.profile = p
	u.descriptor = nil
	u.selected = nil
	u.visibleCommands = nil
	u.list.Clear()
	u.close()
	u.app.SetFocus(u.search)
	return nil
}
