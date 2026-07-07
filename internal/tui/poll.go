package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/status"
)

const (
	pollInterval = 30 * time.Second
	pollTimeout  = 2 * time.Second
)

// pollTickMsg fires every pollInterval to trigger the next probe cycle.
type pollTickMsg struct{}

// hostStatusMsg reports one host's reachability, as probed in the
// background — never persisted, purely an in-memory, best-effort signal.
type hostStatusMsg struct {
	hostID string
	online bool
}

func pollTick() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// probeHostCmd probes one host and reports the result as a hostStatusMsg.
func probeHostCmd(h model.Host) tea.Cmd {
	return func() tea.Msg {
		online := status.Probe(h.Address, h.Port, pollTimeout)
		return hostStatusMsg{hostID: h.ID, online: online}
	}
}

// probeVisibleCmd probes every host currently shown in the host table (not
// the full inventory — repeatedly TCP-probing every host in a large
// inventory on a timer is avoidable network noise against production
// infrastructure). bubbletea runs each returned tea.Cmd concurrently, so
// concurrency here is naturally bounded by how many rows fit on screen at
// once, not by an explicit worker pool.
func (m appModel) probeVisibleCmd() tea.Cmd {
	if len(m.hosts.hosts) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, len(m.hosts.hosts))
	for i, h := range m.hosts.hosts {
		cmds[i] = probeHostCmd(h)
	}
	return tea.Batch(cmds...)
}
