package tui

import tea "github.com/charmbracelet/bubbletea"

// keyToBytes translates a bubbletea key event into the raw bytes a real
// terminal would send for it, for forwarding into an embedded SSH
// session's pty.
//
// appCursor is the session's DECCKM ("application cursor keys") state —
// full-screen ncurses apps (htop, vim, less, ...) enable it on startup
// and then expect arrows/home/end as SS3 sequences (ESC O A) instead of
// the normal CSI form (ESC [ A); their terminfo only maps the SS3 form,
// so sending CSI while the mode is on makes arrow keys dead inside them.
//
// Runes and most control characters need no lookup table at all: bubble
// tea's KeyType constants for them are literally the ASCII control-code
// value (KeyEnter==13, KeyTab==9, KeyEsc==27, KeyCtrlC==3, ...) — see
// bubbletea's key.go — so byte(km.Type) already is the right byte to send.
// Only the handful of keys with no natural single-byte representation
// (arrows, home/end, page up/down, function keys) need explicit
// xterm/VT100 escape sequences.
func keyToBytes(km tea.KeyMsg, appCursor bool) []byte {
	// SS3 ("\x1bO") vs CSI ("\x1b[") prefix for the cursor keys, per
	// DECCKM — the same switch xterm and every other emulator makes.
	cursor := func(final byte) []byte {
		if appCursor {
			return []byte{0x1b, 'O', final}
		}
		return []byte{0x1b, '[', final}
	}
	switch km.Type {
	case tea.KeyRunes:
		return []byte(string(km.Runes))
	case tea.KeySpace:
		return []byte(" ")
	case tea.KeyUp:
		return cursor('A')
	case tea.KeyDown:
		return cursor('B')
	case tea.KeyRight:
		return cursor('C')
	case tea.KeyLeft:
		return cursor('D')
	case tea.KeyCtrlRight:
		return []byte("\x1b[1;5C")
	case tea.KeyCtrlLeft:
		return []byte("\x1b[1;5D")
	case tea.KeyHome:
		return cursor('H')
	case tea.KeyEnd:
		return cursor('F')
	case tea.KeyPgUp:
		return []byte("\x1b[5~")
	case tea.KeyPgDown:
		return []byte("\x1b[6~")
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyInsert:
		return []byte("\x1b[2~")
	case tea.KeyShiftTab:
		return []byte("\x1b[Z")
	case tea.KeyF1:
		return []byte("\x1bOP")
	case tea.KeyF2:
		return []byte("\x1bOQ")
	case tea.KeyF3:
		return []byte("\x1bOR")
	case tea.KeyF4:
		return []byte("\x1bOS")
	case tea.KeyF5:
		return []byte("\x1b[15~")
	case tea.KeyF6:
		return []byte("\x1b[17~")
	case tea.KeyF7:
		return []byte("\x1b[18~")
	case tea.KeyF8:
		return []byte("\x1b[19~")
	case tea.KeyF9:
		return []byte("\x1b[20~")
	case tea.KeyF10:
		return []byte("\x1b[21~")
	case tea.KeyF11:
		return []byte("\x1b[23~")
	case tea.KeyF12:
		return []byte("\x1b[24~")
	default:
		if km.Type >= 0 && km.Type < 256 {
			return []byte{byte(km.Type)}
		}
		// An extended key we don't have an escape sequence for — drop it
		// rather than guess at bytes that might do something unintended
		// on the remote end.
		return nil
	}
}
