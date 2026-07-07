package tui

import "strings"

// emptyStateView is shown when the store has no hosts at all — an
// onboarding hint instead of a blank table.
func emptyStateView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("No hosts yet"))
	b.WriteString("\n\n")
	b.WriteString(hintLine([][2]string{{"a", "add a host manually"}}))
	b.WriteString("\n")
	b.WriteString(hintLine([][2]string{{"i", "import (CSV, JSON, ssh_config, or a live Termius install)"}}))
	b.WriteString("\n")
	b.WriteString(hintLine([][2]string{{"?", "full keybinding help"}}))
	return boxStyle.Render(b.String())
}
