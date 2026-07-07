package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/drkpkg/minissh/internal/model"
)

// groupNode is one visible row in the flattened groups tree: either the
// synthetic "All Hosts" root, or a real model.Group at some depth.
type groupNode struct {
	group       model.Group
	depth       int
	hostCount   int
	isRoot      bool
	hasChildren bool
	expanded    bool
}

// groupsPanel is the left panel: a collapsible tree over the group
// hierarchy, with a host count per node (including nested subgroups).
type groupsPanel struct {
	nodes     []groupNode
	cursor    int
	collapsed map[string]bool
}

func newGroupsPanel(hosts []model.Host, groups []model.Group) groupsPanel {
	p := groupsPanel{collapsed: map[string]bool{}}
	p.rebuild(hosts, groups)
	return p
}

// rebuild recomputes the flattened node list from scratch. Called whenever
// hosts/groups change (import, add, delete, favorite toggle affects counts
// only indirectly, so this is cheap enough to call liberally).
func (p *groupsPanel) rebuild(hosts []model.Host, groups []model.Group) {
	byID := make(map[string]model.Group, len(groups))
	for _, g := range groups {
		byID[g.ID] = g
	}

	// Count hosts per group, including every ancestor (so a parent group's
	// count is the sum of everything nested under it).
	countByGroup := map[string]int{}
	for _, h := range hosts {
		gid := h.GroupID
		seen := map[string]bool{}
		for gid != "" && !seen[gid] {
			seen[gid] = true
			countByGroup[gid]++
			g, ok := byID[gid]
			if !ok {
				break
			}
			gid = g.ParentID
		}
	}

	byParent := map[string][]model.Group{}
	for _, g := range groups {
		byParent[g.ParentID] = append(byParent[g.ParentID], g)
	}
	for parent := range byParent {
		list := byParent[parent]
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	}

	nodes := []groupNode{{isRoot: true, hostCount: len(hosts)}}

	var walk func(parentID string, depth int, visited map[string]bool)
	walk = func(parentID string, depth int, visited map[string]bool) {
		for _, g := range byParent[parentID] {
			if visited[g.ID] {
				continue // defends against malformed cyclic ParentID data
			}
			visited[g.ID] = true
			_, hasChildren := byParent[g.ID]
			expanded := !p.collapsed[g.ID]
			nodes = append(nodes, groupNode{
				group:       g,
				depth:       depth,
				hostCount:   countByGroup[g.ID],
				hasChildren: hasChildren,
				expanded:    expanded,
			})
			if hasChildren && expanded {
				walk(g.ID, depth+1, visited)
			}
		}
	}
	walk("", 1, map[string]bool{})

	p.nodes = nodes
	if p.cursor >= len(nodes) {
		p.cursor = len(nodes) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *groupsPanel) MoveDown() {
	if p.cursor < len(p.nodes)-1 {
		p.cursor++
	}
}

func (p *groupsPanel) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// Selected returns the group at the cursor, or ok=false if the cursor is on
// the "All Hosts" root (meaning: no group filter).
func (p *groupsPanel) Selected() (model.Group, bool) {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return model.Group{}, false
	}
	n := p.nodes[p.cursor]
	if n.isRoot {
		return model.Group{}, false
	}
	return n.group, true
}

// ToggleExpand flips the collapsed state of the group under the cursor.
// No-op on the root or on leaf groups.
func (p *groupsPanel) ToggleExpand() {
	if p.cursor < 0 || p.cursor >= len(p.nodes) {
		return
	}
	n := p.nodes[p.cursor]
	if n.isRoot || !n.hasChildren {
		return
	}
	p.collapsed[n.group.ID] = !p.collapsed[n.group.ID]
}

func (p groupsPanel) View(width int, focused bool) string {
	var b strings.Builder
	b.WriteString(panelHeaderStyle(focused).Render("GROUPS"))
	b.WriteString("\n")
	for i, n := range p.nodes {
		line := groupLineText(n, width)
		switch {
		case i == p.cursor && focused:
			line = selectedLineStyle.Render(line)
		case i == p.cursor:
			line = selectedLineMutedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// groupBreadcrumb walks parent groups to build a "parent > child" string
// for a host's GroupID, e.g. for the details panel.
func groupBreadcrumb(groupID string, groups []model.Group) string {
	byID := make(map[string]model.Group, len(groups))
	for _, g := range groups {
		byID[g.ID] = g
	}

	var parts []string
	seen := map[string]bool{}
	for groupID != "" && !seen[groupID] {
		seen[groupID] = true
		g, ok := byID[groupID]
		if !ok {
			break
		}
		parts = append([]string{g.Name}, parts...)
		groupID = g.ParentID
	}
	return strings.Join(parts, " > ")
}

// descendantGroupIDs returns every group ID nested (at any depth) under
// rootID, used to scope the host table to a group including its subgroups.
func descendantGroupIDs(rootID string, groups []model.Group) map[string]bool {
	byParent := map[string][]model.Group{}
	for _, g := range groups {
		byParent[g.ParentID] = append(byParent[g.ParentID], g)
	}

	result := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		for _, child := range byParent[id] {
			if result[child.ID] {
				continue
			}
			result[child.ID] = true
			walk(child.ID)
		}
	}
	walk(rootID)
	return result
}

func groupLineText(n groupNode, width int) string {
	var left string
	switch {
	case n.isRoot:
		left = "All Hosts"
	default:
		marker := "  "
		if n.hasChildren {
			if n.expanded {
				marker = "▾ "
			} else {
				marker = "▸ "
			}
		}
		left = strings.Repeat("  ", n.depth-1) + marker + n.group.Name
	}

	count := fmt.Sprintf("%d", n.hostCount)
	pad := width - len(left) - len(count)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + count
}
