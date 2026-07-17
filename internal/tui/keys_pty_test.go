package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyToBytesRunes(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")}, false)
	if string(got) != "abc" {
		t.Fatalf("keyToBytes(runes) = %q, want %q", got, "abc")
	}
}

func TestKeyToBytesControlCharactersUseRawASCIIValue(t *testing.T) {
	cases := []struct {
		name string
		typ  tea.KeyType
		want byte
	}{
		{"enter", tea.KeyEnter, 13},
		{"tab", tea.KeyTab, 9},
		{"esc", tea.KeyEsc, 27},
		{"backspace", tea.KeyBackspace, 127},
		{"ctrl+c", tea.KeyCtrlC, 3},
		{"ctrl+a", tea.KeyCtrlA, 1},
		{"ctrl+z", tea.KeyCtrlZ, 26},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := keyToBytes(tea.KeyMsg{Type: tc.typ}, false)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("keyToBytes(%s) = %v, want [%d]", tc.name, got, tc.want)
			}
		})
	}
}

func TestKeyToBytesSpace(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeySpace}, false)
	if string(got) != " " {
		t.Fatalf("keyToBytes(space) = %q, want %q", got, " ")
	}
}

func TestKeyToBytesArrowsAndNavigation(t *testing.T) {
	cases := []struct {
		typ  tea.KeyType
		want string
	}{
		{tea.KeyUp, "\x1b[A"},
		{tea.KeyDown, "\x1b[B"},
		{tea.KeyRight, "\x1b[C"},
		{tea.KeyLeft, "\x1b[D"},
		{tea.KeyHome, "\x1b[H"},
		{tea.KeyEnd, "\x1b[F"},
		{tea.KeyPgUp, "\x1b[5~"},
		{tea.KeyPgDown, "\x1b[6~"},
		{tea.KeyDelete, "\x1b[3~"},
		{tea.KeyInsert, "\x1b[2~"},
		{tea.KeyShiftTab, "\x1b[Z"},
		{tea.KeyCtrlRight, "\x1b[1;5C"},
		{tea.KeyCtrlLeft, "\x1b[1;5D"},
	}
	for _, tc := range cases {
		got := keyToBytes(tea.KeyMsg{Type: tc.typ}, false)
		if string(got) != tc.want {
			t.Fatalf("keyToBytes(%v) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

// With DECCKM (application cursor keys) on — the mode htop/vim/less set on
// startup — arrows and home/end must switch to the SS3 prefix; everything
// else stays as-is.
func TestKeyToBytesAppCursorModeUsesSS3(t *testing.T) {
	cases := []struct {
		typ  tea.KeyType
		want string
	}{
		{tea.KeyUp, "\x1bOA"},
		{tea.KeyDown, "\x1bOB"},
		{tea.KeyRight, "\x1bOC"},
		{tea.KeyLeft, "\x1bOD"},
		{tea.KeyHome, "\x1bOH"},
		{tea.KeyEnd, "\x1bOF"},
		// Unaffected by DECCKM: still CSI.
		{tea.KeyPgUp, "\x1b[5~"},
		{tea.KeyDelete, "\x1b[3~"},
		{tea.KeyCtrlRight, "\x1b[1;5C"},
	}
	for _, tc := range cases {
		got := keyToBytes(tea.KeyMsg{Type: tc.typ}, true)
		if string(got) != tc.want {
			t.Fatalf("keyToBytes(%v, appCursor) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestKeyToBytesFunctionKeys(t *testing.T) {
	cases := []struct {
		typ  tea.KeyType
		want string
	}{
		{tea.KeyF1, "\x1bOP"},
		{tea.KeyF4, "\x1bOS"},
		{tea.KeyF5, "\x1b[15~"},
		{tea.KeyF12, "\x1b[24~"},
	}
	for _, tc := range cases {
		got := keyToBytes(tea.KeyMsg{Type: tc.typ}, false)
		if string(got) != tc.want {
			t.Fatalf("keyToBytes(%v) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestKeyToBytesUnrecognizedExtendedKeyIsDropped(t *testing.T) {
	// KeyF20 is a real, large negative KeyType with no case in the
	// switch/default range check — must not panic or fabricate bytes.
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyF20}, false)
	if got != nil {
		t.Fatalf("expected nil for an unmapped extended key, got %v", got)
	}
}
