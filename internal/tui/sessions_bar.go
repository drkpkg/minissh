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
}

// sessionTabBarView renders the single-line strip of open session tabs
// shown above the HOSTS panel whenever at least one embedded session is
// alive. The current tab only reads as "attached" while inSessionMode —
// when detached (browsing the host table with sessions still running in
// the background), no tab is highlighted, making it visually clear that
// keystrokes are going to the panel, not a session.
func sessionTabBarView(sessions []liveSession, currentIdx int, inSessionMode bool) string {
	tabs := make([]string, len(sessions))
	for i, ls := range sessions {
		label := fmt.Sprintf(" %d:%s ", i+1, ls.host.Label)
		if inSessionMode && i == currentIdx {
			tabs[i] = selectedLineStyle.Render(label)
		} else {
			tabs[i] = subtleStyle.Render(label)
		}
	}
	return strings.Join(tabs, " ") + subtleStyle.Render("  ctrl+←/→ switch · ctrl+\\ detach")
}
