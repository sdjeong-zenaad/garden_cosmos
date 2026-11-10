package ui

import (
	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionModePreference(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	c := garden.ParseHelp([]string{"tx", "gov", "draft-proposal"}, "Draft\nUsage:\n demo tx gov draft-proposal [flags]\n")
	u.selected = c
	u.commandForm()
	if !u.editor.preferInteractive() || !strings.Contains(u.editor.interactionHint.GetText(false), "질문") {
		t.Fatal("draft advice missing")
	}
	_, p := u.pages.GetFrontPage()
	page := p.(*commandPage)
	page.InputHandler()(tcell.NewEventKey(tcell.KeyF6, 0, 0), func(p tview.Primitive) { u.app.SetFocus(p) })
	if b, ok := u.app.GetFocus().(*tview.Button); !ok || b.GetLabel() != "대화형 실행" {
		t.Fatal("F6 did not select recommendation")
	}
	u.editor.modeSelector.SetCurrentOption(1)
	if u.editor.preferInteractive() {
		t.Fatal("explicit normal override ignored")
	}
	u.leaveCommand()
	u.selected = c
	u.commandForm()
	if u.editor.executionMode != "normal" || u.editor.preferInteractive() {
		t.Fatal("override not restored")
	}
	u.leaveCommand()
	u.descriptor.Home = "/other-chain"
	u.selected = c
	u.commandForm()
	if u.editor.executionMode != "auto" || !u.editor.preferInteractive() {
		t.Fatal("override leaked across chain home")
	}
	u.editor.modeSelector.SetCurrentOption(2)
	u.editor.modeSelector.SetCurrentOption(0)
	var saved map[string]string
	if err := garden.Load(filepath.Join(u.dir, "execution-modes.json"), &saved); err != nil {
		t.Fatal(err)
	}
	if _, ok := saved[u.editor.preferenceKey+"\n"+strings.Join(c.Path, " ")]; ok {
		t.Fatal("auto mode did not remove override")
	}
}

func TestInteractionFlagUpdates(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	u.selected = &garden.Command{Path: []string{"tx", "bank", "send"}, Usage: "demo tx bank send [flags]", Flags: []garden.Flag{{Name: "yes", Type: "bool", Default: "false"}, {Name: "keyring-backend", Type: "string", Default: "os"}}}
	u.commandForm()
	if !u.editor.preferInteractive() {
		t.Fatal("default confirmation missing")
	}
	u.editor.values["flag:yes"] = garden.FieldValue{Enabled: true, Value: "true"}
	u.updateCommandPreview()
	if !u.editor.preferInteractive() {
		t.Fatal("--yes hid keyring requirement")
	}
	u.editor.values["flag:keyring-backend"] = garden.FieldValue{Enabled: true, Value: "test"}
	u.updateCommandPreview()
	if u.editor.preferInteractive() || !strings.Contains(u.editor.interactionHint.GetText(false), "일반 실행 가능") {
		t.Fatal("hint did not follow flags")
	}
	// Failed preference persistence must not silently apply an unsaved override.
	if err := os.Mkdir(filepath.Join(u.dir, "execution-modes.json"), 0700); err != nil {
		t.Fatal(err)
	}
	u.editor.modeSelector.SetCurrentOption(2)
	if u.editor.executionMode != "auto" {
		t.Fatal("failed save applied override")
	}
}
