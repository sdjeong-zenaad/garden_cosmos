package ui

import (
	"strings"

	"garden_cosmos/internal/garden"
)

func (u *UI) commands() []*garden.Command {
	if u.descriptor == nil {
		return nil
	}
	q := strings.ToLower(u.search.GetText())
	var out []*garden.Command
	for _, c := range garden.Flatten(u.descriptor.Root) {
		// Keep command groups in the descriptor for traversal, but do not
		// offer help-only parents as executable commands in the browser.
		if len(c.Children) > 0 || strings.Contains(c.Usage, "[command]") {
			continue
		}
		if strings.Contains(strings.ToLower(strings.Join(c.Path, " ")+" "+c.Summary), q) {
			out = append(out, c)
		}
	}
	return out
}
func (u *UI) populate(string) {
	// Publish the exact row mapping before Clear/AddItem invoke callbacks.
	u.visibleCommands = u.commands()
	u.list.Clear()
	for _, c := range u.visibleCommands {
		name := strings.Join(c.Path, " ")
		if name == "" {
			name = "(root)"
		}
		u.list.AddItem(name, c.Summary, 0, nil)
	}
	u.selectIndex(0)
}
func (u *UI) selectIndex(i int) {
	cs := u.visibleCommands
	if i < 0 || i >= len(cs) {
		u.selected = nil
		u.help.SetText("")
		return
	}
	u.selected = cs[i]
	u.help.SetText(u.selected.Help)
	u.help.ScrollToBeginning()
}
