package tui

import (
	"fmt"
	"strings"

	"github.com/drkpkg/minissh/internal/model"
	"github.com/drkpkg/minissh/internal/sshsession"
)

// liveSession is one open embedded SSH session, whether or not it's the
// one currently focused in the HOSTS panel.
type liveSession struct {
	host model.Host
	sess *sshsession.Session
	// ended is true once the process has exited abnormally (a fast,
	// non-zero exit nobody asked for — see connectFailureThreshold) and is
	// frozen awaiting dismissal: its last screen stays visible with a
	// banner instead of the tab disappearing.
	ended    bool
	exitCode int
	waitErr  error
}

// sessionTabBarView renders the single-line strip of open session tabs
// shown above the HOSTS panel whenever at least one embedded session is
// alive. The current tab only reads as "attached" while inSessionMode —
// when detached (browsing the host table with sessions still running in
// the background), no tab is highlighted, making it visually clear that
// keystrokes are going to the panel, not a session. Ended (failed) tabs
// get a distinct marker and color so a failure is visible even when a
// different tab is focused.
func sessionTabBarView(sessions []liveSession, currentIdx int, inSessionMode bool) string {
	tabs := make([]string, len(sessions))
	for i, ls := range sessions {
		label := fmt.Sprintf(" %d:%s", i+1, ls.host.Label)
		if ls.ended {
			label += " ✗"
		}
		label += " "
		switch {
		case inSessionMode && i == currentIdx:
			tabs[i] = selectedLineStyle.Render(label)
		case ls.ended:
			tabs[i] = errorStyle.Render(label)
		default:
			tabs[i] = subtleStyle.Render(label)
		}
	}
	return strings.Join(tabs, " ") + subtleStyle.Render("  ctrl+←/→ switch · ctrl+\\ detach")
}
