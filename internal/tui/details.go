package tui

import (
	"fmt"
	"strings"

	"github.com/drkpkg/minissh/internal/model"
)

// detailsView renders the right panel for the given host, or a hint when
// nothing is selected. groups is used to resolve the group breadcrumb.
func detailsView(host *model.Host, groups []model.Group, width int, focused bool) string {
	var b strings.Builder
	b.WriteString(panelHeaderStyle(focused).Render("DETAILS"))
	b.WriteString("\n\n")

	if host == nil {
		b.WriteString(subtleStyle.Render("Select a host to see details here."))
		return b.String()
	}

	b.WriteString(titleStyle.Render(host.Label))
	b.WriteString("\n\n")

	row := func(label, value string) {
		if value == "" {
			value = subtleStyle.Render("—")
		}
		b.WriteString(subtleStyle.Render(fmt.Sprintf("%-8s", label)))
		b.WriteString(value)
		b.WriteString("\n")
	}

	row("Host", host.Address)
	row("Port", fmt.Sprintf("%d", host.Port))
	row("User", host.Username)
	row("Auth", authDetail(host.Identity))
	row("Group", groupBreadcrumb(host.GroupID, groups))
	row("Tags", strings.Join(host.Tags, ", "))

	if host.Favorite {
		b.WriteString("\n")
		b.WriteString(favoriteStyle.Render("★ Favorite"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if host.LastConnectedAt.IsZero() {
		row("Last", "never")
	} else {
		row("Last", host.LastConnectedAt.Format("2006-01-02 15:04"))
	}

	if host.Notes != "" {
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("Notes"))
		b.WriteString("\n")
		b.WriteString(wrapText(host.Notes, width))
		b.WriteString("\n")
	}

	return b.String()
}

func authDetail(id model.Identity) string {
	switch id.Kind {
	case model.IdentityKey:
		if id.KeyPath != "" {
			return "🔑 " + id.KeyPath
		}
		return "🔑 key"
	case model.IdentityPassword:
		return "password"
	case model.IdentityAgent:
		return "ssh-agent"
	default:
		return ""
	}
}

// wrapText does simple greedy word wrapping to width (minimum 20), no
// external dependency needed for this one panel.
func wrapText(s string, width int) string {
	if width < 20 {
		width = 20
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}

	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		if lineLen > 0 && lineLen+1+len(w) > width {
			b.WriteString("\n")
			lineLen = 0
		} else if i > 0 {
			b.WriteString(" ")
			lineLen++
		}
		b.WriteString(w)
		lineLen += len(w)
	}
	return b.String()
}
