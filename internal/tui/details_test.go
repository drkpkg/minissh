package tui

import (
	"strings"
	"testing"

	"github.com/drkpkg/minissh/internal/model"
)

func TestDetailsViewNilHostShowsHint(t *testing.T) {
	view := detailsView(nil, nil, 40, true)
	if !strings.Contains(view, "Select a host") {
		t.Fatalf("expected hint text, got: %s", view)
	}
}

func TestDetailsViewShowsHostFields(t *testing.T) {
	groups := []model.Group{{ID: "g1", Name: "prod"}}
	h := &model.Host{
		Label:    "web-1",
		Address:  "10.0.0.1",
		Port:     22,
		Username: "root",
		GroupID:  "g1",
		Tags:     []string{"prod", "web"},
		Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "~/.ssh/id_ed25519"},
		Favorite: true,
	}
	view := detailsView(h, groups, 40, true)

	for _, want := range []string{"web-1", "10.0.0.1", "root", "prod", "web", "id_ed25519", "Favorite", "never"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected details view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestAuthDetailAllKinds(t *testing.T) {
	cases := []struct {
		id   model.Identity
		want string
	}{
		{model.Identity{Kind: model.IdentityKey, KeyPath: "/x"}, "/x"},
		{model.Identity{Kind: model.IdentityKey}, "key"},
		{model.Identity{Kind: model.IdentityPassword}, "password"},
		{model.Identity{Kind: model.IdentityAgent}, "ssh-agent"},
	}
	for _, tc := range cases {
		if got := authDetail(tc.id); !strings.Contains(got, tc.want) {
			t.Fatalf("authDetail(%+v) = %q, want to contain %q", tc.id, got, tc.want)
		}
	}
}

func TestWrapTextBreaksAtWidth(t *testing.T) {
	wrapped := wrapText("the quick brown fox jumps over the lazy dog", 20)
	for _, line := range strings.Split(wrapped, "\n") {
		if len(line) > 20 {
			t.Fatalf("line exceeds width 20: %q", line)
		}
	}
}

func TestWrapTextEnforcesMinimumWidthFloor(t *testing.T) {
	// wrapText refuses to wrap narrower than 20 cols even if asked to —
	// avoids degenerate near-single-character-per-line rendering in a
	// squeezed details panel.
	wrapped := wrapText("the quick brown fox jumps over the lazy dog", 5)
	for _, line := range strings.Split(wrapped, "\n") {
		if len(line) > 20 {
			t.Fatalf("line exceeds the 20-col floor: %q", line)
		}
	}
}

func TestWrapTextEmpty(t *testing.T) {
	if got := wrapText("", 20); got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}
