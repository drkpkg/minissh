package tui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/drkpkg/minissh/internal/model"
)

// sortColumn is which host-table column drives the current sort order.
type sortColumn int

const (
	sortByName sortColumn = iota
	sortByHost
	sortByUser
	sortByStatus
	sortColumnCount
)

func (c sortColumn) String() string {
	switch c {
	case sortByHost:
		return "Host"
	case sortByUser:
		return "User"
	case sortByStatus:
		return "Status"
	default:
		return "Name"
	}
}

// hostsTable is the center panel: a sortable table over the currently
// scoped (group-filtered, search-filtered) host list. statuses is populated
// by the background poller (Phase 4) — nil/missing entries render as
// unknown rather than offline, so an empty map before the first poll cycle
// doesn't falsely claim every host is down.
type hostsTable struct {
	tbl      table.Model
	hosts    []model.Host // parallel to tbl.Rows(): same order, same index
	statuses map[string]bool
	sortCol  sortColumn
	sortAsc  bool
	// preserveOrder skips the sortCol-driven sort — set while a search
	// query is active, so fuzzy-match relevance order (best match first)
	// isn't immediately undone by re-sorting alphabetically.
	preserveOrder bool
}

func newHostsTable() hostsTable {
	t := table.New(
		table.WithColumns(hostTableColumns(100)),
		table.WithFocused(false),
		table.WithHeight(10),
	)
	st := table.DefaultStyles()
	st.Header = st.Header.
		Bold(true).
		Foreground(colorMuted).
		BorderBottom(true).
		BorderForeground(colorBorder)
	st.Selected = selectedLineStyle
	t.SetStyles(st)
	return hostsTable{tbl: t, statuses: map[string]bool{}, sortCol: sortByName, sortAsc: true}
}

func hostTableColumns(width int) []table.Column {
	if width < 40 {
		width = 40
	}
	nameW := width * 32 / 100
	hostW := width * 28 / 100
	userW := width * 18 / 100
	statusW := 9
	favW := width - nameW - hostW - userW - statusW - 3 // 3 ~ column separators
	if favW < 3 {
		favW = 3
	}
	return []table.Column{
		{Title: "NAME", Width: nameW},
		{Title: "HOST", Width: hostW},
		{Title: "USER", Width: userW},
		{Title: "STATUS", Width: statusW},
		{Title: "★", Width: favW},
	}
}

func (h *hostsTable) SetSize(width, height int) {
	h.tbl.SetColumns(hostTableColumns(width))
	h.tbl.SetWidth(width)
	h.tbl.SetHeight(height)
}

func (h *hostsTable) Focus() { h.tbl.Focus() }
func (h *hostsTable) Blur()  { h.tbl.Blur() }

// SetPreserveOrder controls whether SetHosts re-sorts by the active sort
// column (false, default) or keeps the order it was given (true — used
// while a search query is active).
func (h *hostsTable) SetPreserveOrder(preserve bool) {
	h.preserveOrder = preserve
}

// SetHosts replaces the visible host set (already scoped by the caller —
// group selection, search match, etc.) and re-applies the current sort.
func (h *hostsTable) SetHosts(hosts []model.Host) {
	h.hosts = append([]model.Host(nil), hosts...)
	h.refresh()
}

// SetStatuses updates the live reachability map without touching which
// hosts are shown, then re-applies the sort (status may be the active sort
// column).
func (h *hostsTable) SetStatuses(statuses map[string]bool) {
	h.statuses = statuses
	h.refresh()
}

func (h *hostsTable) CycleSort() {
	h.sortCol = (h.sortCol + 1) % sortColumnCount
	h.sortAsc = true
	h.refresh()
}

// ToggleSortDirection flips ascending/descending on the current column.
func (h *hostsTable) ToggleSortDirection() {
	h.sortAsc = !h.sortAsc
	h.refresh()
}

func (h *hostsTable) refresh() {
	if !h.preserveOrder {
		sort.SliceStable(h.hosts, h.less)
	}

	rows := make([]table.Row, len(h.hosts))
	for i, hst := range h.hosts {
		rows[i] = table.Row{
			hst.Label,
			hostAddressColumn(hst),
			hst.Username,
			statusBadge(h.statuses, hst.ID),
			favoriteBadge(hst.Favorite),
		}
	}

	cursor := h.tbl.Cursor()
	h.tbl.SetRows(rows)
	if cursor < len(rows) {
		h.tbl.SetCursor(cursor)
	}
}

func (h *hostsTable) less(i, j int) bool {
	var a, b string
	switch h.sortCol {
	case sortByHost:
		a, b = h.hosts[i].Address, h.hosts[j].Address
	case sortByUser:
		a, b = h.hosts[i].Username, h.hosts[j].Username
	case sortByStatus:
		a, b = statusSortKey(h.statuses, h.hosts[i].ID), statusSortKey(h.statuses, h.hosts[j].ID)
	default:
		a, b = h.hosts[i].Label, h.hosts[j].Label
	}
	if a == b {
		a, b = h.hosts[i].Label, h.hosts[j].Label // stable tiebreaker
	}
	if h.sortAsc {
		return a < b
	}
	return a > b
}

// Selected returns the host at the table's cursor.
func (h *hostsTable) Selected() (model.Host, bool) {
	idx := h.tbl.Cursor()
	if idx < 0 || idx >= len(h.hosts) {
		return model.Host{}, false
	}
	return h.hosts[idx], true
}

func (h *hostsTable) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	h.tbl, cmd = h.tbl.Update(msg)
	return cmd
}

func (h hostsTable) View(focused bool, title string) string {
	header := panelHeaderStyle(focused).Render(title)
	return header + "\n" + h.tbl.View()
}

func hostAddressColumn(h model.Host) string {
	if h.Port != 0 && h.Port != 22 {
		return fmt.Sprintf("%s:%d", h.Address, h.Port)
	}
	return h.Address
}

// authTag is a short at-a-glance indicator of how a host authenticates.
func authTag(kind model.IdentityKind) string {
	switch kind {
	case model.IdentityKey:
		return "🔑"
	case model.IdentityPassword:
		return "[pwd]"
	case model.IdentityAgent:
		return "[agent]"
	default:
		return ""
	}
}

// statusBadge renders a host's live reachability, or "—" if it hasn't been
// probed yet (never claims "offline" for a host that simply hasn't been
// checked).
//
// Deliberately plain text, no lipgloss styling: bubbles/table truncates
// cell values with runewidth.Truncate, which isn't ANSI-escape-aware —
// embedding a styled (color-coded) string here gets it truncated mid
// escape-sequence, corrupting the rendered terminal output. Meaning is
// carried by the glyph (●/○/—) instead of color for anything that goes
// into a table cell; full color is used everywhere else (details panel,
// status bar) where the string is rendered directly, not through the
// table's truncation path.
func statusBadge(statuses map[string]bool, hostID string) string {
	online, known := statuses[hostID]
	switch {
	case !known:
		return "—"
	case online:
		return "● online"
	default:
		return "○ offline"
	}
}

func statusSortKey(statuses map[string]bool, hostID string) string {
	online, known := statuses[hostID]
	switch {
	case !known:
		return "1"
	case online:
		return "0"
	default:
		return "2"
	}
}

// favoriteBadge is plain text for the same reason statusBadge is — see its
// comment.
func favoriteBadge(fav bool) string {
	if fav {
		return "★"
	}
	return ""
}
