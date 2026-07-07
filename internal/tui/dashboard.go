package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/drkpkg/minissh/internal/model"
)

// dashboardView is the home screen shown before you start browsing:
// favorites, recently connected, recently added, aggregate stats, and
// hosts known to be offline from the last poll cycle.
func dashboardView(hosts []model.Host, groups []model.Group, statuses map[string]bool) string {
	var b strings.Builder

	b.WriteString(dashboardSection("★ FAVORITES", filterHosts(hosts, func(h model.Host) bool { return h.Favorite }),
		func(h model.Host) string { return fmt.Sprintf("%-20s %s", h.Label, h.Address) }))

	b.WriteString(dashboardSection("⏱ RECENT", recentlyConnected(hosts, 5),
		func(h model.Host) string { return fmt.Sprintf("%-20s %s", h.Label, relativeTime(h.LastConnectedAt)) }))

	b.WriteString(panelHeaderStyle(false).Render("STATS"))
	b.WriteString("\n")
	online, offline := 0, 0
	for _, h := range hosts {
		if o, known := statuses[h.ID]; known {
			if o {
				online++
			} else {
				offline++
			}
		}
	}
	b.WriteString(subtleStyle.Render(fmt.Sprintf("  %d hosts · %d groups", len(hosts), len(groups))))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("  %d online · %d offline · %d unknown", online, offline, len(hosts)-online-offline)))
	b.WriteString("\n\n")

	b.WriteString(dashboardSection("⚠ OFFLINE", filterHosts(hosts, func(h model.Host) bool {
		o, known := statuses[h.ID]
		return known && !o
	}), func(h model.Host) string { return fmt.Sprintf("%-20s %s", h.Label, h.Address) }))

	b.WriteString(dashboardSection("+ RECENTLY ADDED", recentlyAdded(hosts, 5),
		func(h model.Host) string { return h.Label }))

	return b.String()
}

func dashboardSection(title string, hosts []model.Host, line func(model.Host) string) string {
	var b strings.Builder
	b.WriteString(panelHeaderStyle(false).Render(title))
	b.WriteString("\n")
	if len(hosts) == 0 {
		b.WriteString(subtleStyle.Render("  none"))
		b.WriteString("\n\n")
		return b.String()
	}
	const max = 5
	for i, h := range hosts {
		if i >= max {
			b.WriteString(subtleStyle.Render(fmt.Sprintf("  ...and %d more", len(hosts)-max)))
			b.WriteString("\n")
			break
		}
		b.WriteString("  " + line(h))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func filterHosts(hosts []model.Host, pred func(model.Host) bool) []model.Host {
	var out []model.Host
	for _, h := range hosts {
		if pred(h) {
			out = append(out, h)
		}
	}
	return out
}

func recentlyConnected(hosts []model.Host, limit int) []model.Host {
	var connected []model.Host
	for _, h := range hosts {
		if !h.LastConnectedAt.IsZero() {
			connected = append(connected, h)
		}
	}
	sort.Slice(connected, func(i, j int) bool {
		return connected[i].LastConnectedAt.After(connected[j].LastConnectedAt)
	})
	if len(connected) > limit {
		connected = connected[:limit]
	}
	return connected
}

// recentlyAdded approximates "recently added" from insertion order —
// there's no CreatedAt field, but store.UpsertHost appends new hosts to
// the end of the slice, so the tail (reversed) is a reasonable proxy.
func recentlyAdded(hosts []model.Host, limit int) []model.Host {
	n := len(hosts)
	start := n - limit
	if start < 0 {
		start = 0
	}
	tail := append([]model.Host(nil), hosts[start:]...)
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	return tail
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
