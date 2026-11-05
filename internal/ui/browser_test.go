package ui

import (
	"testing"

	"garden_cosmos/internal/garden"

	"github.com/rivo/tview"
)

func TestCommandSelectionUsesRenderedRows(t *testing.T) {
	first := &garden.Command{Path: []string{"first"}, Summary: "first", Help: "first help"}
	second := &garden.Command{Path: []string{"second"}, Summary: "second", Help: "second help"}
	root := &garden.Command{Children: []*garden.Command{first, second}}
	u := &UI{app: tview.NewApplication(), pages: tview.NewPages()}
	u.build()
	u.descriptor = &garden.Descriptor{Root: root}
	u.populate("")
	// Selection must refer to what was rendered, never a newly traversed tree.
	root.Children = []*garden.Command{second, first}
	u.selectIndex(1)
	if u.selected != second || u.help.GetText(false) != "second help" {
		t.Fatal("selection no longer matches visible row")
	}
	u.populate("")
	if u.selected != second {
		t.Fatal("refresh did not replace visible rows")
	}
	u.search.SetText("no match")
	if len(u.visibleCommands) != 0 || u.selected != nil {
		t.Fatal("empty search kept stale selection")
	}
}
