package tui

import (
	"net"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/drkpkg/minissh/internal/model"
)

func TestProbeHostCmdReportsOnline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	cmd := probeHostCmd(model.Host{ID: "h1", Address: addr.IP.String(), Port: addr.Port})
	msg := cmd()
	hsm, ok := msg.(hostStatusMsg)
	if !ok {
		t.Fatalf("expected hostStatusMsg, got %T", msg)
	}
	if hsm.hostID != "h1" || !hsm.online {
		t.Fatalf("expected h1 online, got %+v", hsm)
	}
}

func TestProbeVisibleCmdEmptyReturnsNil(t *testing.T) {
	m := newAppModel(nil, nil)
	if cmd := m.probeVisibleCmd(); cmd != nil {
		t.Fatal("expected nil cmd when there are no visible hosts")
	}
}

func TestProbeVisibleCmdReturnsBatchForHosts(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()
	if cmd := m.probeVisibleCmd(); cmd == nil {
		t.Fatal("expected a non-nil batched cmd for a non-empty host table")
	}
}

func TestHostStatusMsgUpdatesStatusesAndPropagatesToTable(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	target, _ := m.hosts.Selected()

	updated, cmd := m.Update(hostStatusMsg{hostID: target.ID, online: true})
	mm := updated.(appModel)
	if cmd != nil {
		t.Fatal("expected no follow-up cmd from a single status update")
	}
	if online, known := mm.statuses[target.ID]; !known || !online {
		t.Fatalf("expected statuses[%q]=true, got known=%v online=%v", target.ID, known, online)
	}
}

func TestPollTickMsgReschedulesAndReprobes(t *testing.T) {
	hosts, groups := testHostsAndGroups()
	m := newAppModel(hosts, groups)
	m.applySizes()

	_, cmd := m.Update(pollTickMsg{})
	if cmd == nil {
		t.Fatal("expected a batched cmd (next tick + reprobe) from pollTickMsg")
	}
}

var _ tea.Msg = pollTickMsg{}
var _ tea.Msg = hostStatusMsg{}
