package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/danieluremix/minissh/internal/model"
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

// hostFormField is one input field in the Add/Edit host modal.
type hostFormField int

const (
	fieldLabel hostFormField = iota
	fieldAddress
	fieldPort
	fieldUsername
	fieldGroup
	fieldKeyPath
	fieldCount
)

var hostFormLabels = [fieldCount]string{
	fieldLabel:    "Label",
	fieldAddress:  "Address",
	fieldPort:     "Port",
	fieldUsername: "Username",
	fieldGroup:    "Group",
	fieldKeyPath:  "Key path",
}

// hostForm is the Add/Edit Host modal's state: a small set of text inputs,
// tab-cycled. Only editable here: label/address/port/username/group/key
// path — favorite/notes/tags/last-connected are left untouched on edit.
type hostForm struct {
	editing  bool
	hostID   string
	inputs   [fieldCount]textinput.Model
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
		fieldKeyPath:  h.Identity.KeyPath,
	}
	for i := range f.inputs {
		ti := textinput.New()
		ti.Placeholder = hostFormLabels[i]
		ti.SetValue(values[i])
		f.inputs[i] = ti
	}
	f.inputs[fieldLabel].Focus()
	return f
}

func portValue(port int) string {
	if port == 0 {
		return "22"
	}
	return strconv.Itoa(port)
}

func (f *hostForm) Next() {
	f.inputs[f.focusIdx].Blur()
	f.focusIdx = (f.focusIdx + 1) % int(fieldCount)
	f.inputs[f.focusIdx].Focus()
}

func (f *hostForm) Prev() {
	f.inputs[f.focusIdx].Blur()
	f.focusIdx = (f.focusIdx - 1 + int(fieldCount)) % int(fieldCount)
	f.inputs[f.focusIdx].Focus()
}

func (f *hostForm) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.focusIdx], cmd = f.inputs[f.focusIdx].Update(msg)
	return cmd
}

// toHost validates the form and converts it into a model.Host. GroupID is
// left unset — the caller resolves the group-name input into an ID via
// store.UpsertGroup, since that requires the live store.
func (f hostForm) toHost() (model.Host, error) {
	address := strings.TrimSpace(f.inputs[fieldAddress].Value())
	if address == "" {
		return model.Host{}, fmt.Errorf("address is required")
	}

	port := 22
	if p := strings.TrimSpace(f.inputs[fieldPort].Value()); p != "" {
		v, err := strconv.Atoi(p)
		if err != nil || v <= 0 {
			return model.Host{}, fmt.Errorf("invalid port %q", p)
		}
		port = v
	}

	identity := model.Identity{Kind: model.IdentityAgent}
	if kp := strings.TrimSpace(f.inputs[fieldKeyPath].Value()); kp != "" {
		identity = model.Identity{Kind: model.IdentityKey, KeyPath: kp}
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
	}, nil
}

func (f hostForm) View() string {
	title := "Add Host"
	if f.editing {
		title = "Edit Host"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	for i, ti := range f.inputs {
		b.WriteString(subtleStyle.Render(fmt.Sprintf("%-10s", hostFormLabels[i])))
		b.WriteString(ti.View())
		b.WriteString("\n")
	}
	if f.err != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(f.err))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(hintLine([][2]string{{"tab", "next field"}, {"enter", "save"}, {"esc", "cancel"}}))
	return boxStyle.Render(b.String())
}

func deleteConfirmView(host model.Host) string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("Delete host?"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("%s (%s) will be permanently removed.", host.Label, host.Address))
	b.WriteString("\n\n")
	b.WriteString(hintLine([][2]string{{"y", "confirm"}, {"any other key", "cancel"}}))
	return boxStyle.Render(b.String())
}
