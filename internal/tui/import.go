package tui

import (
	"github.com/charmbracelet/bubbles/list"

	"github.com/drkpkg/minissh/internal/sources"
)

// sourceItem adapts sources.Source to bubbles/list.Item for the import
// wizard's source picker.
type sourceItem struct {
	source sources.Source
}

func (i sourceItem) Title() string       { return i.source.Name }
func (i sourceItem) Description() string { return i.source.Description }
func (i sourceItem) FilterValue() string { return i.source.Name + " " + i.source.Description }

func buildSourceItems() []list.Item {
	items := make([]list.Item, len(sources.All))
	for i, s := range sources.All {
		items[i] = sourceItem{source: s}
	}
	return items
}

// themedDelegate returns a list.DefaultDelegate whose selection highlight
// uses the app's accent color instead of the library default.
func themedDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(colorAccent).BorderLeftForeground(colorAccent)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(colorAccent)
	return d
}
