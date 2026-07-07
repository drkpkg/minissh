package tui

import (
	"strconv"
	"strings"
)

// statusBar renders the bottom bar: mode, filter, counts, selection, and
// context-sensitive shortcuts that change with which panel has focus.
func statusBar(f focus, hostCount, totalCount int, filterQuery string, selectedLabel string) string {
	left := []string{"NORMAL"}
	if filterQuery != "" {
		left = append(left, "filter: "+filterQuery)
	} else {
		left = append(left, "filter: none")
	}
	left = append(left, pluralCount(hostCount, "host"))
	if totalCount != hostCount {
		left[len(left)-1] += " of " + pluralCount(totalCount, "host")
	}
	if selectedLabel != "" {
		left = append(left, "selected: "+selectedLabel)
	}

	info := subtleStyle.Render(strings.Join(left, "  │  "))
	hints := hintLine(hintsForFocus(f))
	return info + "\n" + hints
}

func pluralCount(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

func hintsForFocus(f focus) [][2]string {
	base := [][2]string{
		{"h/l", "panel"}, {"tab", "cycle"}, {"/", "search"}, {"?", "help"}, {"q", "quit"},
	}
	switch f {
	case focusGroups:
		return append([][2]string{{"j/k", "move"}, {"space", "expand"}, {"enter", "drill in"}}, base...)
	case focusHosts:
		return append([][2]string{
			{"j/k", "move"}, {"enter", "connect"}, {"E", "connect full-screen"},
			{"space", "favorite"}, {"s", "sort"}, {"a", "add"}, {"e", "edit"}, {"d", "delete"},
		}, base...)
	default:
		return base
	}
}
