package tui

import (
	"testing"

	"github.com/danieluremix/minissh/internal/model"
)

func TestNewGroupsPanelCountsIncludeNestedSubgroups(t *testing.T) {
	groups := []model.Group{
		{ID: "g-prod", Name: "prod"},
		{ID: "g-web", Name: "web", ParentID: "g-prod"},
		{ID: "g-db", Name: "db", ParentID: "g-prod"},
	}
	hosts := []model.Host{
		{Label: "a", GroupID: "g-web"},
		{Label: "b", GroupID: "g-web"},
		{Label: "c", GroupID: "g-db"},
		{Label: "d"}, // ungrouped
	}

	p := newGroupsPanel(hosts, groups)

	var root, prod, web, db groupNode
	for _, n := range p.nodes {
		switch {
		case n.isRoot:
			root = n
		case n.group.ID == "g-prod":
			prod = n
		case n.group.ID == "g-web":
			web = n
		case n.group.ID == "g-db":
			db = n
		}
	}

	if root.hostCount != 4 {
		t.Fatalf("expected root count 4, got %d", root.hostCount)
	}
	if prod.hostCount != 3 {
		t.Fatalf("expected prod count 3 (web+db), got %d", prod.hostCount)
	}
	if web.hostCount != 2 {
		t.Fatalf("expected web count 2, got %d", web.hostCount)
	}
	if db.hostCount != 1 {
		t.Fatalf("expected db count 1, got %d", db.hostCount)
	}
}

func TestNewGroupsPanelHandlesCyclicParentIDsWithoutHanging(t *testing.T) {
	// Malformed data: a group listing itself as its own parent. rebuild
	// must not infinite-loop — if it did, go test's own -timeout would
	// eventually kill the run and fail the package.
	groups := []model.Group{
		{ID: "g1", Name: "self-parent", ParentID: "g1"},
	}
	p := newGroupsPanel(nil, groups)
	if len(p.nodes) != 1 {
		t.Fatalf("expected only the root node (self-parent group is unreachable from root), got %d nodes", len(p.nodes))
	}
}

func TestGroupsPanelExpandCollapseChangesVisibleNodes(t *testing.T) {
	groups := []model.Group{
		{ID: "g-prod", Name: "prod"},
		{ID: "g-web", Name: "web", ParentID: "g-prod"},
	}
	p := newGroupsPanel(nil, groups)
	if len(p.nodes) != 3 { // root, prod, web
		t.Fatalf("expected 3 nodes expanded, got %d", len(p.nodes))
	}

	p.cursor = 1 // "prod"
	p.ToggleExpand()
	p.rebuild(nil, groups)
	if len(p.nodes) != 2 { // root, prod (web hidden)
		t.Fatalf("expected 2 nodes after collapsing prod, got %d", len(p.nodes))
	}

	p.ToggleExpand()
	p.rebuild(nil, groups)
	if len(p.nodes) != 3 {
		t.Fatalf("expected 3 nodes after re-expanding prod, got %d", len(p.nodes))
	}
}

func TestGroupsPanelSelectedReturnsFalseOnRoot(t *testing.T) {
	p := newGroupsPanel(nil, nil)
	if _, ok := p.Selected(); ok {
		t.Fatal("expected Selected() to report false on the root node")
	}
}

func TestGroupsPanelMoveUpDownClampsAtBounds(t *testing.T) {
	groups := []model.Group{{ID: "g1", Name: "a"}, {ID: "g2", Name: "b"}}
	p := newGroupsPanel(nil, groups)

	p.MoveUp() // already at 0, should stay
	if p.cursor != 0 {
		t.Fatalf("expected cursor clamped at 0, got %d", p.cursor)
	}

	last := len(p.nodes) - 1
	for i := 0; i < 10; i++ {
		p.MoveDown()
	}
	if p.cursor != last {
		t.Fatalf("expected cursor clamped at %d, got %d", last, p.cursor)
	}
}

func TestDescendantGroupIDs(t *testing.T) {
	groups := []model.Group{
		{ID: "g-prod", Name: "prod"},
		{ID: "g-web", Name: "web", ParentID: "g-prod"},
		{ID: "g-api", Name: "api", ParentID: "g-web"},
		{ID: "g-staging", Name: "staging"},
	}
	got := descendantGroupIDs("g-prod", groups)
	if !got["g-web"] || !got["g-api"] {
		t.Fatalf("expected g-web and g-api as descendants of g-prod, got %v", got)
	}
	if got["g-staging"] {
		t.Fatal("expected g-staging not to be a descendant of g-prod")
	}
}

func TestGroupBreadcrumb(t *testing.T) {
	groups := []model.Group{
		{ID: "g-prod", Name: "prod"},
		{ID: "g-web", Name: "web", ParentID: "g-prod"},
	}
	if got := groupBreadcrumb("g-web", groups); got != "prod > web" {
		t.Fatalf("groupBreadcrumb = %q", got)
	}
	if got := groupBreadcrumb("", groups); got != "" {
		t.Fatalf("groupBreadcrumb(\"\") = %q, want empty", got)
	}
}
