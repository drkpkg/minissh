package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/drkpkg/minissh/internal/model"
)

// renderModal centers boxed content within a width x height canvas. This is
// a full-screen-replace modal (the whole screen becomes the modal while
// it's open) rather than a true dimmed-but-visible overlay on the
// dashboard: compositing over a live ANSI-styled background requires
// visual-column-aware splicing, which is real risk of the same class of
// subtle rendering-corruption bug already found and fixed in this redesign
// (see statusBadge's comment in hosts_table.go) — not worth it for a
// cosmetic gain. Full-screen-replace is also what the import wizard
// already did before this file existed.
func renderModal(width, height int, content string) string {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// hostFormField is one field in the Add/Edit host modal. fieldAuthMode is
// not a textinput — it's a 3-way agent/key/password selector, cycled with
// ←/→. fieldSecret is a single textinput whose meaning (key path vs
// password, masked or not) depends on the current auth mode.
type hostFormField int

const (
	fieldLabel hostFormField = iota
	fieldAddress
	fieldPort
	fieldUsername
	fieldGroup
	fieldAuthMode
	fieldSecret
	fieldCount
)

var hostFormLabels = [fieldCount]string{
	fieldLabel:    "Label",
	fieldAddress:  "Address",
	fieldPort:     "Port",
	fieldUsername: "Username",
	fieldGroup:    "Group",
	fieldAuthMode: "Auth",
}

// authMode is the selected authentication method in the Add/Edit modal.
type authMode int

const (
	authAgent authMode = iota
	authKey
	authPassword
	authModeCount
)

func (a authMode) String() string {
	switch a {
	case authKey:
		return "key"
	case authPassword:
		return "password"
	default:
		return "agent"
	}
}

// hostForm is the Add/Edit Host modal's state: a small set of text inputs
// plus the auth-mode selector, tab-cycled. Only editable here:
// label/address/port/username/group/auth — favorite/notes/tags/
// last-connected are left untouched on edit.
type hostForm struct {
	editing bool
	hostID  string
	// originalAuthPassword is true when editing a host that already uses
	// password auth — leaving the (always-blank) password field empty on
	// save then means "keep the existing stored password" instead of
	// "clear it" or failing validation.
	originalAuthPassword bool

	inputs   [fieldCount]textinput.Model // fieldAuthMode's entry is unused
	authMode authMode
	focusIdx int
	err      string
}

func newHostForm(editing bool, h model.Host, groupName string) hostForm {
	f := hostForm{editing: editing, hostID: h.ID}

	values := [fieldCount]string{
		fieldLabel:    h.Label,
		fieldAddress:  h.Address,
		fieldPort:     portValue(h.Port),
		fieldUsername: h.Username,
		fieldGroup:    groupName,
	}
	switch h.Identity.Kind {
	case model.IdentityKey:
		f.authMode = authKey
		values[fieldSecret] = h.Identity.KeyPath
	case model.IdentityPassword:
		f.authMode = authPassword
		f.originalAuthPassword = true
		// Deliberately left blank — never re-displays a stored secret.
	default:
		f.authMode = authAgent
	}

	for i := range f.inputs {
		if hostFormField(i) == fieldAuthMode {
			continue
		}
		ti := textinput.New()
		ti.Placeholder = hostFormLabels[i]
		ti.SetValue(values[i])
		f.inputs[i] = ti
	}
	f.syncSecretField()
	f.inputs[fieldLabel].Focus()
	return f
}

func portValue(port int) string {
	if port == 0 {
		return "22"
	}
	return strconv.Itoa(port)
}

// syncSecretField updates the fieldSecret input's placeholder and masking
// to match the current auth mode. Called whenever authMode changes.
func (f *hostForm) syncSecretField() {
	ti := &f.inputs[fieldSecret]
	switch f.authMode {
	case authKey:
		ti.Placeholder = "~/.ssh/id_ed25519"
		ti.EchoMode = textinput.EchoNormal
	case authPassword:
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
		if f.editing && f.originalAuthPassword {
			ti.Placeholder = "(unchanged — leave blank to keep current password)"
		} else {
			ti.Placeholder = "password"
		}
	default:
		ti.Placeholder = ""
		ti.EchoMode = textinput.EchoNormal
	}
}

// CycleAuthMode moves the auth-mode selector by delta (±1, wrapping).
func (f *hostForm) CycleAuthMode(delta int) {
	n := int(authModeCount)
	f.authMode = authMode((int(f.authMode) + delta + n) % n)
	f.syncSecretField()
}

func (f *hostForm) focusCurrent() {
	if hostFormField(f.focusIdx) != fieldAuthMode {
		f.inputs[f.focusIdx].Focus()
	}
}

func (f *hostForm) blurCurrent() {
	if hostFormField(f.focusIdx) != fieldAuthMode {
		f.inputs[f.focusIdx].Blur()
	}
}

// Next/Prev skip fieldSecret entirely when auth mode is agent — there's
// nothing to fill in for agent auth.
func (f *hostForm) Next() {
	f.blurCurrent()
	next := f.focusIdx
	for {
		next = (next + 1) % int(fieldCount)
		if hostFormField(next) == fieldSecret && f.authMode == authAgent {
			continue
		}
		break
	}
	f.focusIdx = next
	f.focusCurrent()
}

func (f *hostForm) Prev() {
	f.blurCurrent()
	prev := f.focusIdx
	for {
		prev = (prev - 1 + int(fieldCount)) % int(fieldCount)
		if hostFormField(prev) == fieldSecret && f.authMode == authAgent {
			continue
		}
		break
	}
	f.focusIdx = prev
	f.focusCurrent()
}

func (f *hostForm) Update(msg tea.Msg) tea.Cmd {
	if hostFormField(f.focusIdx) == fieldAuthMode {
		return nil // handled by the caller via CycleAuthMode (←/→), not a textinput
	}
	var cmd tea.Cmd
	f.inputs[f.focusIdx], cmd = f.inputs[f.focusIdx].Update(msg)
	return cmd
}

// toHost validates the form and converts it into a model.Host plus the
// plaintext password to store in the keychain (empty unless auth mode is
// password and a new password was actually entered — the caller decides
// what "empty" means: for a fresh add it's always required, for an edit of
// an already-password host it means "keep the existing one"). GroupID is
// left unset — the caller resolves the group-name input into an ID via
// store.UpsertGroup, since that requires the live store.
func (f hostForm) toHost() (model.Host, string, error) {
	address := strings.TrimSpace(f.inputs[fieldAddress].Value())
	if address == "" {
		return model.Host{}, "", fmt.Errorf("address is required")
	}

	port := 22
	if p := strings.TrimSpace(f.inputs[fieldPort].Value()); p != "" {
		v, err := strconv.Atoi(p)
		if err != nil || v <= 0 {
			return model.Host{}, "", fmt.Errorf("invalid port %q", p)
		}
		port = v
	}

	var identity model.Identity
	var password string
	switch f.authMode {
	case authKey:
		kp := strings.TrimSpace(f.inputs[fieldSecret].Value())
		if kp == "" {
			return model.Host{}, "", fmt.Errorf("key path is required for key auth")
		}
		identity = model.Identity{Kind: model.IdentityKey, KeyPath: kp}
	case authPassword:
		pw := f.inputs[fieldSecret].Value()
		if pw == "" && (!f.editing || !f.originalAuthPassword) {
			return model.Host{}, "", fmt.Errorf("password is required for password auth")
		}
		identity = model.Identity{Kind: model.IdentityPassword}
		password = pw
	default:
		identity = model.Identity{Kind: model.IdentityAgent}
	}

	label := strings.TrimSpace(f.inputs[fieldLabel].Value())
	if label == "" {
		label = address
	}

	return model.Host{
		ID:       f.hostID,
		Label:    label,
		Address:  address,
		Port:     port,
		Username: strings.TrimSpace(f.inputs[fieldUsername].Value()),
		Identity: identity,
	}, password, nil
}

func (f hostForm) View() string {
	title := "Add Host"
	if f.editing {
		title = "Edit Host"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	for i := 0; i < int(fieldCount); i++ {
		field := hostFormField(i)
		if field == fieldSecret && f.authMode == authAgent {
			continue // nothing to show for agent auth
		}

		label := f.fieldLabel(field)
		focused := i == f.focusIdx

		var value string
		if field == fieldAuthMode {
			value = authModeSelectorView(f.authMode, focused)
		} else {
			value = f.inputs[i].View()
		}

		b.WriteString(subtleStyle.Render(fmt.Sprintf("%-10s", label)))
		b.WriteString(value)
		b.WriteString("\n")
	}

	if f.err != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(f.err))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(hintLine([][2]string{{"tab", "next field"}, {"←/→", "auth mode"}, {"enter", "save"}, {"esc", "cancel"}}))
	return boxStyle.Render(b.String())
}

func (f hostForm) fieldLabel(field hostFormField) string {
	if field == fieldSecret {
		switch f.authMode {
		case authKey:
			return "Key path"
		case authPassword:
			return "Password"
		default:
			return ""
		}
	}
	return hostFormLabels[field]
}

func authModeSelectorView(mode authMode, focused bool) string {
	opts := []authMode{authAgent, authKey, authPassword}
	parts := make([]string, len(opts))
	for i, o := range opts {
		mark := "( )"
		if o == mode {
			mark = "(•)"
		}
		text := mark + " " + o.String()
		if o == mode && focused {
			text = keyStyle.Render(text)
		}
		parts[i] = text
	}
	line := strings.Join(parts, "  ")
	if focused {
		line += subtleStyle.Render("  ←/→")
	}
	return line
}

func deleteConfirmView(host model.Host) string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("Delete host?"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s (%s) will be permanently removed.", host.Label, host.Address)
	b.WriteString("\n\n")
	b.WriteString(hintLine([][2]string{{"y", "confirm"}, {"any other key", "cancel"}}))
	return boxStyle.Render(b.String())
}
