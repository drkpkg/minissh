package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/sahilm/fuzzy"

	"github.com/drkpkg/minissh/internal/model"
)

// searchField is which host attribute a live search query matches against.
type searchField int

const (
	searchAll searchField = iota
	searchName
	searchAddress
	searchUser
	searchTags
	searchGroup
	searchFieldCount
)

func (f searchField) String() string {
	switch f {
	case searchName:
		return "name"
	case searchAddress:
		return "address"
	case searchUser:
		return "user"
	case searchTags:
		return "tag"
	case searchGroup:
		return "group"
	default:
		return "all"
	}
}

// searchHosts fuzzy-filters hosts against query, scoped to field, in
// relevance order (best match first). An empty query returns hosts
// unchanged, in their given order.
func searchHosts(hosts []model.Host, groups []model.Group, query string, field searchField) []model.Host {
	if strings.TrimSpace(query) == "" {
		return hosts
	}

	haystack := make([]string, len(hosts))
	for i, h := range hosts {
		haystack[i] = searchKey(h, groups, field)
	}

	matches := fuzzy.Find(query, haystack)
	out := make([]model.Host, len(matches))
	for i, m := range matches {
		out[i] = hosts[m.Index]
	}
	return out
}

// searchBarView renders the inline search input shown above the panels
// while a search is active.
func searchBarView(ti textinput.Model, field searchField) string {
	label := fmt.Sprintf("/ search (%s, tab to cycle field)  ", field)
	return keyStyle.Render(label) + ti.View()
}

func searchKey(h model.Host, groups []model.Group, field searchField) string {
	switch field {
	case searchName:
		return h.Label
	case searchAddress:
		return h.Address
	case searchUser:
		return h.Username
	case searchTags:
		return strings.Join(h.Tags, " ")
	case searchGroup:
		return groupBreadcrumb(h.GroupID, groups)
	default:
		return strings.Join([]string{
			h.Label, h.Address, h.Username,
			strings.Join(h.Tags, " "),
			groupBreadcrumb(h.GroupID, groups),
		}, " ")
	}
}
