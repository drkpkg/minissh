package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/danieluremix/minissh/internal/connect"
	"github.com/danieluremix/minissh/internal/importer"
	"github.com/danieluremix/minissh/internal/importflow"
	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/sources"
	"github.com/danieluremix/minissh/internal/store"
)

// focus is which of the three panels h/l/tab currently target.
type focus int

const (
	focusGroups focus = iota
	focusHosts
	focusDetails
	focusCount
)

// overlay is a full-input-capturing state layered on top of the main
// 3-panel view. Today this is only the import wizard; Phase 3 turns these
// into proper centered modals without changing this state machine.
type overlay int

const (
	overlayNone overlay = iota
	overlaySourcePicker
	overlayFilePrompt
	overlayConfirm
	overlayAddEdit
	overlayDeleteConfirm
)

type appModel struct {
	focus   focus
	overlay overlay
	// homeView is true before you've started browsing hosts — shows the
	// dashboard (favorites/recent/stats/offline) in the center panel
	// instead of the host table. Any interaction with the host table
	// switches it off for the rest of the session.
	homeView bool

	width, height             int
	groupsWidth, detailsWidth int
	bodyHeight                int

	allHosts  []model.Host
	allGroups []model.Group
	groups    groupsPanel
	hosts     hostsTable
	statuses  map[string]bool

	searching   bool
	searchQuery string
	searchField searchField
	searchInput textinput.Model

	hostForm     hostForm
	deleteTarget *model.Host

	sourceList list.Model
	filePrompt textinput.Model

	chosenSource sources.Source
	importResult *importer.Result
	importErr    error
}

func newAppModel(hosts []model.Host, groups []model.Group) appModel {
	ht := newHostsTable()
	ht.SetHosts(hosts)
	ht.Focus()

	sl := list.New(buildSourceItems(), themedDelegate(), 0, 0)
	sl.Title = "Import from..."
	sl.Styles.Title = titleStyle
	sl.SetShowStatusBar(false)
	sl.SetFilteringEnabled(false)
	// Handled explicitly in updateSourcePicker instead — q/esc mean "back
	// to the host list" here, not "quit the app".
	sl.DisableQuitKeybindings()

	return appModel{
		focus:      focusHosts,
		homeView:   len(hosts) > 0,
		allHosts:   hosts,
		allGroups:  groups,
		groups:     newGroupsPanel(hosts, groups),
		hosts:      ht,
		statuses:   map[string]bool{},
		sourceList: sl,
	}
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(pollTick(), m.probeVisibleCmd())
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = wsm.Width, wsm.Height
		m.applySizes()
		return m, nil
	}

	switch tm := msg.(type) {
	case pollTickMsg:
		return m, tea.Batch(pollTick(), m.probeVisibleCmd())
	case hostStatusMsg:
		m.statuses[tm.hostID] = tm.online
		m.hosts.SetStatuses(m.statuses)
		return m, nil
	case connectFinishedMsg:
		// The ssh session (whatever its outcome) has ended and bubbletea
		// has restored the terminal to us — reload so the dashboard
		// reflects the just-recorded LastConnectedAt.
		if s, err := store.Load(); err == nil {
			m.refreshFromStore(s)
		}
		return m, nil
	}

	if m.overlay != overlayNone {
		switch m.overlay {
		case overlaySourcePicker:
			return m.updateSourcePicker(msg)
		case overlayFilePrompt:
			return m.updateFilePrompt(msg)
		case overlayConfirm:
			return m.updateConfirm(msg)
		case overlayAddEdit:
			return m.updateAddEdit(msg)
		case overlayDeleteConfirm:
			return m.updateDeleteConfirm(msg)
		}
	}

	if m.searching {
		return m.updateSearch(msg)
	}

	return m.updateMain(msg)
}

func (m *appModel) applySizes() {
	groupsW := m.width * 22 / 100
	detailsW := m.width * 26 / 100
	hostsW := m.width - groupsW - detailsW - 6 // 3 panels x 2 cols border overhead
	if hostsW < 20 {
		hostsW = 20
	}
	bodyH := m.height - 4 // status bar (2 lines) + panel header/border allowance
	if bodyH < 5 {
		bodyH = 5
	}
	m.groupsWidth, m.detailsWidth, m.bodyHeight = groupsW, detailsW, bodyH
	m.hosts.SetSize(hostsW, bodyH-2)
	m.sourceList.SetSize(m.width, m.height)
}

func (m appModel) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "h", "shift+tab":
			m.moveFocus(-1)
			return m, nil
		case "l", "tab":
			m.moveFocus(1)
			return m, nil
		case "i":
			m.overlay = overlaySourcePicker
			return m, nil
		case "/":
			ti := textinput.New()
			ti.Placeholder = "search..."
			ti.Focus()
			m.searchInput = ti
			m.searching = true
			m.homeView = false
			return m, textinput.Blink
		}
	}

	switch m.focus {
	case focusGroups:
		return m.updateGroupsFocus(msg)
	case focusHosts:
		return m.updateHostsFocus(msg)
	default:
		return m, nil
	}
}

func (m *appModel) moveFocus(delta int) {
	m.hosts.Blur()
	next := (int(m.focus) + delta + int(focusCount)) % int(focusCount)
	m.focus = focus(next)
	if m.focus == focusHosts {
		m.hosts.Focus()
	}
}

func (m appModel) updateGroupsFocus(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.homeView = false
	switch km.String() {
	case "j", "down":
		m.groups.MoveDown()
		m.applyFilters()
	case "k", "up":
		m.groups.MoveUp()
		m.applyFilters()
	case "enter":
		m.applyFilters()
		m.moveFocus(1)
	case " ":
		m.groups.ToggleExpand()
		m.groups.rebuild(m.allHosts, m.allGroups) // collapsed state only takes effect on rebuild
	}
	return m, nil
}

// applyFilters scopes the host table to the currently selected group (and
// its subgroups, or every host for the root "All Hosts" node), then applies
// the active search query on top of that scope.
func (m *appModel) applyFilters() {
	var scoped []model.Host
	if g, ok := m.groups.Selected(); ok {
		descendants := descendantGroupIDs(g.ID, m.allGroups)
		for _, h := range m.allHosts {
			if h.GroupID == g.ID || descendants[h.GroupID] {
				scoped = append(scoped, h)
			}
		}
	} else {
		scoped = m.allHosts
	}

	m.hosts.SetPreserveOrder(m.searchQuery != "")
	m.hosts.SetHosts(searchHosts(scoped, m.allGroups, m.searchQuery, m.searchField))
}

func (m appModel) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyEsc:
			m.searching = false
			m.searchQuery = ""
			m.searchField = searchAll
			m.applyFilters()
			return m, nil
		case tea.KeyEnter:
			m.searching = false
			m.focus = focusHosts
			m.hosts.Focus()
			return m, nil
		case tea.KeyTab:
			m.searchField = (m.searchField + 1) % searchFieldCount
			m.applyFilters()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchQuery = m.searchInput.Value()
	m.applyFilters()
	return m, cmd
}

func (m appModel) updateHostsFocus(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		m.homeView = false // any interaction with the host panel leaves the dashboard
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "enter":
			if h, ok := m.hosts.Selected(); ok {
				return m.connectTo(h)
			}
			return m, nil
		case " ":
			if h, ok := m.hosts.Selected(); ok {
				m.toggleFavorite(h.ID)
			}
			return m, nil
		case "s":
			m.hosts.CycleSort()
			return m, nil
		case "a":
			m.hostForm = newHostForm(false, model.Host{}, "")
			m.overlay = overlayAddEdit
			return m, textinput.Blink
		case "e":
			if h, ok := m.hosts.Selected(); ok {
				m.hostForm = newHostForm(true, h, groupBreadcrumb(h.GroupID, m.allGroups))
				m.overlay = overlayAddEdit
				return m, textinput.Blink
			}
			return m, nil
		case "d":
			if h, ok := m.hosts.Selected(); ok {
				sel := h
				m.deleteTarget = &sel
				m.overlay = overlayDeleteConfirm
			}
			return m, nil
		}
	}
	cmd := m.hosts.Update(msg)
	return m, cmd
}

func (m appModel) updateAddEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyEsc:
			m.overlay = overlayNone
			return m, nil
		case tea.KeyTab:
			m.hostForm.Next()
			return m, nil
		case tea.KeyShiftTab:
			m.hostForm.Prev()
			return m, nil
		case tea.KeyEnter:
			return m.saveHostForm()
		}
	}
	cmd := m.hostForm.Update(msg)
	return m, cmd
}

// saveHostForm validates and persists the Add/Edit modal. On edit, it
// mutates the existing host in place by ID (not via store.UpsertHost's
// label+address matching, which would silently create a duplicate if the
// label or address itself was the thing being edited) and preserves fields
// the form doesn't expose (tags, favorite, last-connected, notes).
func (m appModel) saveHostForm() (tea.Model, tea.Cmd) {
	h, err := m.hostForm.toHost()
	if err != nil {
		m.hostForm.err = err.Error()
		return m, nil
	}

	s, err := store.Load()
	if err != nil {
		m.hostForm.err = err.Error()
		return m, nil
	}
	groupName := strings.TrimSpace(m.hostForm.inputs[fieldGroup].Value())
	h.GroupID = store.UpsertGroup(s, groupName, "")

	if m.hostForm.editing {
		found := false
		for i := range s.Hosts {
			if s.Hosts[i].ID == m.hostForm.hostID {
				h.ID = s.Hosts[i].ID
				h.Tags = s.Hosts[i].Tags
				h.Favorite = s.Hosts[i].Favorite
				h.LastConnectedAt = s.Hosts[i].LastConnectedAt
				h.Notes = s.Hosts[i].Notes
				s.Hosts[i] = h
				found = true
				break
			}
		}
		if !found {
			m.hostForm.err = "host no longer exists"
			return m, nil
		}
	} else {
		h.ID = ""
		store.UpsertHost(s, h)
	}

	if err := store.Save(s); err != nil {
		m.hostForm.err = err.Error()
		return m, nil
	}

	m.refreshFromStore(s)
	m.overlay = overlayNone
	return m, nil
}

func (m appModel) updateDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if km.String() == "y" {
		return m.confirmDelete()
	}
	m.overlay = overlayNone
	m.deleteTarget = nil
	return m, nil
}

func (m appModel) confirmDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		m.overlay = overlayNone
		return m, nil
	}
	s, err := store.Load()
	if err != nil {
		m.overlay = overlayNone
		m.deleteTarget = nil
		return m, nil
	}
	store.DeleteHostByID(s, m.deleteTarget.ID)
	if err := store.Save(s); err != nil {
		m.overlay = overlayNone
		m.deleteTarget = nil
		return m, nil
	}

	m.refreshFromStore(s)
	m.overlay = overlayNone
	m.deleteTarget = nil
	return m, nil
}

// refreshFromStore reloads the in-memory groups/hosts panels from s after
// a mutation (add/edit/delete/import), so the UI reflects it immediately.
func (m *appModel) refreshFromStore(s *model.Store) {
	m.allHosts = s.Hosts
	m.allGroups = s.Groups
	m.groups.rebuild(s.Hosts, s.Groups)
	m.applyFilters()
}

// connectFinishedMsg is delivered once the suspended ssh session (started
// by connectTo) exits and bubbletea has restored the terminal to minissh.
type connectFinishedMsg struct{ err error }

// connectTo suspends the TUI and hands the terminal to a real ssh child
// process via tea.ExecProcess, instead of quitting the program the way
// exec-replacing (internal/connect.Exec, used by the CLI) would. When the
// ssh session ends — for any reason, including a non-zero exit, which
// happens for plenty of legitimate reasons in a shell session — bubbletea
// restores the terminal and minissh resumes right where it left off.
func (m appModel) connectTo(h model.Host) (tea.Model, tea.Cmd) {
	cmd, err := connect.Command(h)
	if err != nil {
		return m, nil
	}

	// Record before launching, not after: "last connected" then means
	// "last connection attempt," which doesn't require guessing whether a
	// given ssh exit code meant the connection itself failed.
	if s, err := store.Load(); err == nil {
		if store.RecordConnected(s, h.ID, time.Now()) {
			_ = store.Save(s) // best-effort; a failure here shouldn't block connecting
		}
	}

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return connectFinishedMsg{err: err}
	})
}

// toggleFavorite flips the favorite flag both in the persisted store and in
// the in-memory host list backing the current view, so the table's ★
// column updates immediately without a reload.
func (m *appModel) toggleFavorite(hostID string) {
	s, err := store.Load()
	if err != nil {
		return
	}
	newFav := false
	for i := range s.Hosts {
		if s.Hosts[i].ID == hostID {
			newFav = !s.Hosts[i].Favorite
		}
	}
	if !store.SetFavorite(s, hostID, newFav) {
		return
	}
	if err := store.Save(s); err != nil {
		return
	}
	for i := range m.allHosts {
		if m.allHosts[i].ID == hostID {
			m.allHosts[i].Favorite = newFav
		}
	}
	m.applyFilters()
}

func (m appModel) updateSourcePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q":
			m.overlay = overlayNone
			return m, nil
		case "enter":
			it, ok := m.sourceList.SelectedItem().(sourceItem)
			if !ok {
				return m, nil
			}
			m.chosenSource = it.source
			if m.chosenSource.RequiresFile {
				ti := textinput.New()
				ti.Placeholder = "/path/to/file"
				ti.Width = 60
				if m.width > 10 {
					ti.Width = m.width - 4
				}
				ti.Focus()
				m.filePrompt = ti
				m.overlay = overlayFilePrompt
				return m, textinput.Blink
			}
			return m.runImport("")
		}
	}

	var cmd tea.Cmd
	m.sourceList, cmd = m.sourceList.Update(msg)
	return m, cmd
}

func (m appModel) updateFilePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyEsc:
			m.overlay = overlaySourcePicker
			return m, nil
		case tea.KeyEnter:
			return m.runImport(strings.TrimSpace(m.filePrompt.Value()))
		}
	}

	var cmd tea.Cmd
	m.filePrompt, cmd = m.filePrompt.Update(msg)
	return m, cmd
}

// runImport executes the chosen source and moves to the confirm screen,
// whether it succeeded or not (errors are shown there rather than crashing
// the program).
func (m appModel) runImport(path string) (tea.Model, tea.Cmd) {
	res, err := m.chosenSource.Run(path)
	m.importResult = res
	m.importErr = err
	m.overlay = overlayConfirm
	return m, nil
}

func (m appModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.importErr != nil {
		m.overlay = overlayNone
		m.importErr = nil
		m.importResult = nil
		return m, nil
	}
	if km.String() == "y" || km.String() == "enter" {
		return m.confirmImport()
	}
	m.overlay = overlayNone
	m.importResult = nil
	return m, nil
}

// confirmImport persists the already-run import result and refreshes the
// groups/hosts panels in place so they reflect it without a restart.
func (m appModel) confirmImport() (tea.Model, tea.Cmd) {
	keysDir, err := store.KeysDir()
	if err != nil {
		m.importErr = err
		return m, nil
	}
	if _, err := importflow.Persist(m.importResult, importflow.Options{
		KeysDir:      keysDir,
		StoreSecrets: !m.chosenSource.RequiresFile,
	}); err != nil {
		m.importErr = err
		return m, nil
	}

	s, err := store.Load()
	if err != nil {
		m.importErr = err
		return m, nil
	}

	m.refreshFromStore(s)
	m.overlay = overlayNone
	m.importResult = nil
	return m, nil
}

func (m appModel) View() string {
	switch m.overlay {
	case overlaySourcePicker:
		return m.sourceList.View()
	case overlayFilePrompt:
		return renderModal(m.width, m.height, m.filePromptView())
	case overlayConfirm:
		return renderModal(m.width, m.height, m.confirmView())
	case overlayAddEdit:
		return renderModal(m.width, m.height, m.hostForm.View())
	case overlayDeleteConfirm:
		if m.deleteTarget != nil {
			return renderModal(m.width, m.height, deleteConfirmView(*m.deleteTarget))
		}
		return m.mainView()
	default:
		return m.mainView()
	}
}

func (m appModel) mainView() string {
	if len(m.allHosts) == 0 {
		return renderModal(m.width, m.height, emptyStateView())
	}

	selectedHost, hasSelection := m.hosts.Selected()
	var detailsHost *model.Host
	if hasSelection {
		h := selectedHost
		detailsHost = &h
	}

	groupsBox := panelBorderStyle(m.focus == focusGroups).
		Width(m.groupsWidth).Height(m.bodyHeight).
		Render(m.groups.View(m.groupsWidth, m.focus == focusGroups))

	var hostsContent string
	if m.homeView {
		header := panelHeaderStyle(m.focus == focusHosts).Render("DASHBOARD")
		hostsContent = header + "\n" + dashboardView(m.allHosts, m.allGroups, m.statuses)
	} else {
		hostsContent = m.hosts.View(m.focus == focusHosts, fmt.Sprintf("HOSTS (%d)", len(m.hosts.hosts)))
	}
	hostsBox := panelBorderStyle(m.focus == focusHosts).
		Height(m.bodyHeight).
		Render(hostsContent)

	detailsBox := panelBorderStyle(m.focus == focusDetails).
		Width(m.detailsWidth).Height(m.bodyHeight).
		Render(detailsView(detailsHost, m.allGroups, m.detailsWidth, m.focus == focusDetails))

	body := lipgloss.JoinHorizontal(lipgloss.Top, groupsBox, hostsBox, detailsBox)

	selLabel := ""
	if hasSelection {
		selLabel = selectedHost.Label
	}
	bar := statusBar(m.focus, len(m.hosts.hosts), len(m.allHosts), m.searchQuery, selLabel)

	var top string
	if m.searching {
		top = searchBarView(m.searchInput, m.searchField) + "\n"
	}

	return top + body + "\n" + bar
}

func (m appModel) filePromptView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Import from: " + m.chosenSource.Name))
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("File path:"))
	b.WriteString("\n")
	b.WriteString(m.filePrompt.View())
	b.WriteString("\n\n")
	b.WriteString(hintLine([][2]string{{"enter", "continue"}, {"esc", "back"}}))
	return boxStyle.Render(b.String())
}

func (m appModel) confirmView() string {
	if m.importErr != nil {
		body := errorStyle.Render("Import failed: "+m.importErr.Error()) +
			"\n\n" + hintLine([][2]string{{"any key", "back"}})
		return boxStyle.Render(body)
	}

	res := m.importResult
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Found %d host(s) via %s", len(res.Hosts), m.chosenSource.Name)))
	b.WriteString("\n\n")

	const preview = 15
	for i, h := range res.Hosts {
		if i >= preview {
			b.WriteString(subtleStyle.Render(fmt.Sprintf("  ...and %d more", len(res.Hosts)-preview)))
			b.WriteString("\n")
			break
		}
		target := h.Address
		if h.Username != "" {
			target = h.Username + "@" + target
		}
		b.WriteString(fmt.Sprintf("  %-20s %-30s port %d\n", h.Label, target, h.Port))
	}
	if len(res.Skipped) > 0 {
		b.WriteString("\n")
		b.WriteString(warningStyle.Render(fmt.Sprintf("%d entries skipped (see CLI output for detail).", len(res.Skipped))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(hintLine([][2]string{{"y/enter", "confirm"}, {"any other key", "cancel"}}))
	return boxStyle.Render(b.String())
}

// hintLine renders a "key action  •  key action" footer, e.g. "esc back".
func hintLine(pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = keyStyle.Render(p[0]) + " " + subtleStyle.Render(p[1])
	}
	return strings.Join(parts, subtleStyle.Render("  •  "))
}

// Run displays the dashboard and blocks until the user quits. Connecting to
// a host suspends the TUI (via tea.ExecProcess) rather than exiting it —
// minissh stays open and returns to the dashboard once the ssh session
// ends, instead of quitting every time you connect.
func Run(hosts []model.Host, groups []model.Group) error {
	m := newAppModel(hosts, groups)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}
