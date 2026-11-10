package ui

import (
	"os"
	"path/filepath"
	"strings"

	"garden_cosmos/internal/garden"
	"github.com/rivo/tview"
)

// Overrides are scoped to the executable, chain home, and command, not its
// arguments. Keeping them separate from descriptors preserves them on refresh.
func (u *UI) executionMode(ed *commandEditor) (*tview.DropDown, error) {
	path := filepath.Join(u.dir, "execution-modes.json")
	modes := map[string]string{}
	if err := garden.Load(path, &modes); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if modes == nil {
		modes = map[string]string{}
	}
	key := ed.preferenceKey + "\n" + strings.Join(ed.command.Path, " ")
	choices := []string{"auto", "normal", "interactive"}
	index := 0
	for i, mode := range choices {
		if modes[key] == mode {
			index = i
		}
	}
	ed.executionMode = choices[index]
	drop := tview.NewDropDown().SetLabel("기본 실행 (F6): ").SetOptions([]string{"자동 판단", "일반 실행", "대화형 실행"}, nil).SetCurrentOption(index)
	drop.SetListStyles(comboNormal, comboSelected)
	restoring := false
	drop.SetSelectedFunc(func(_ string, i int) {
		if restoring || i < 0 || i >= len(choices) {
			return
		}
		old := ed.executionMode
		mode := choices[i]
		if mode == "auto" {
			delete(modes, key)
		} else {
			modes[key] = mode
		}
		if err := garden.Save(path, modes); err != nil {
			modes[key] = old
			ed.feedback.SetText(tview.Escape(err.Error()))
			restoring = true
			for j, value := range choices {
				if value == old {
					drop.SetCurrentOption(j)
				}
			}
			restoring = false
			return
		}
		ed.executionMode = mode
		ed.updateInteraction()
	})
	return drop, nil
}

func (ed *commandEditor) updateInteraction() {
	if ed.interactionHint == nil {
		return
	}
	advice := garden.ResolveInteraction(ed.command, ed.values)
	label := advice.Label()
	if ed.executionMode == "normal" {
		label = "사용자 지정: 일반 실행"
	}
	if ed.executionMode == "interactive" {
		label = "사용자 지정: 대화형 실행"
	}
	ed.interactionHint.SetText(label + " · " + strings.Join(advice.Reasons, " / "))
}
func (ed *commandEditor) preferInteractive() bool {
	switch ed.executionMode {
	case "normal":
		return false
	case "interactive":
		return true
	default:
		return garden.ResolveInteraction(ed.command, ed.values).PreferInteractive()
	}
}
