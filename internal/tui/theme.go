package tui

import "github.com/charmbracelet/lipgloss"

// Palette: blue (primary accent/focus), cyan (secondary/info), green
// (success/online), amber (warning), red (sparingly — hard errors and
// destructive confirmation only), subtle gray (everything inactive/muted).
// Colors communicate meaning, not decoration — no per-tag/per-group
// rainbow coloring anywhere in this app.
var (
	colorAccent    = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"} // blue
	colorSecondary = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"} // cyan
	colorSuccess   = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"} // green
	colorWarning   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"} // amber
	colorError     = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"} // red, used sparingly
	colorMuted     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"} // subtle gray
	colorBorder    = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#3F3F46"}
	colorBorderDim = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#27272A"}
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Background(colorAccent).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	subtleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorError)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	infoStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	groupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorMuted).
				PaddingLeft(2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	// selectedLineStyle marks the cursor row in a focused panel.
	selectedLineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorAccent)

	// selectedLineMutedStyle marks the cursor row in an unfocused panel —
	// visible (so you don't lose your place when you tab away) but clearly
	// secondary to the focused panel's selection.
	selectedLineMutedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorBorder)

	statusOnlineStyle  = successStyle
	statusOfflineStyle = subtleStyle
	favoriteStyle      = warningStyle
)

// panelHeaderStyle returns the section-header style for a panel, brighter
// when that panel has focus.
func panelHeaderStyle(focused bool) lipgloss.Style {
	if focused {
		return lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	}
	return lipgloss.NewStyle().Bold(true).Foreground(colorMuted)
}

// panelBorderStyle returns the outer border style for a panel, accented
// when focused and dim otherwise — the primary way the interface
// communicates which panel h/l/tab will act on.
func panelBorderStyle(focused bool) lipgloss.Style {
	if focused {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent)
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorderDim)
}
