package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/drkpkg/minissh/internal/model"
	"github.com/drkpkg/minissh/internal/store"
)

// ImportJSON is a best-effort parser for Termius-style JSON exports. Unlike
// ImportCSV (which targets Termius's documented, stable CSV columns), the
// JSON export schema is not officially documented, so this walks the tree
// tolerantly: it recognizes a handful of common field-name aliases and
// container shapes (Vault > Groups > Hosts, or a flat host array), skipping
// anything it can't confidently map instead of failing the whole import.
func ImportJSON(r io.Reader) (*Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &Result{}, nil
	}

	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	res := &Result{Secrets: map[string]string{}}
	tmpStore := &model.Store{}
	walkJSON(root, "", tmpStore, res)
	res.Groups = tmpStore.Groups
	return res, nil
}

var (
	jsonLabelKeys     = []string{"label", "name", "title", "alias"}
	jsonAddressKeys   = []string{"address", "hostname", "host", "ip", "ipaddress", "ip_address"}
	jsonPortKeys      = []string{"port"}
	jsonProtocolKeys  = []string{"protocol", "type"}
	jsonUsernameKeys  = []string{"username", "user"}
	jsonPasswordKeys  = []string{"password"}
	jsonKeyPathKeys   = []string{"key", "keypath", "identityfile", "identity"}
	jsonTagsKeys      = []string{"tags"}
	jsonHostListKeys  = []string{"hosts", "items"}
	jsonGroupListKeys = []string{"groups", "folders", "children", "subgroups"}
)

// walkJSON recursively descends a decoded JSON tree looking for host and
// group records under any of the alias key sets above. A map is treated as
// a container (group) node if it has a host-list or group-list key; its own
// group ID (used for both its direct hosts and its nested subgroups) is
// derived from its name field, falling back to the parent's group ID for
// unnamed wrapper nodes (e.g. a top-level vault object).
func walkJSON(node interface{}, parentGroupID string, tmpStore *model.Store, res *Result) {
	switch v := node.(type) {
	case []interface{}:
		for _, elem := range v {
			walkJSON(elem, parentGroupID, tmpStore, res)
		}
	case map[string]interface{}:
		lower := lowerKeyMap(v)

		hasHostList := matchKey(lower, jsonHostListKeys) != ""
		hasGroupList := matchKey(lower, jsonGroupListKeys) != ""

		if hasHostList || hasGroupList {
			ownGroupID := parentGroupID
			if name, ok := stringField(lower, jsonLabelKeys); ok && name != "" {
				ownGroupID = store.UpsertGroup(tmpStore, name, parentGroupID)
			}
			for _, key := range jsonHostListKeys {
				if list, ok := lower[key]; ok {
					walkJSON(list, ownGroupID, tmpStore, res)
				}
			}
			for _, key := range jsonGroupListKeys {
				if list, ok := lower[key]; ok {
					walkJSON(list, ownGroupID, tmpStore, res)
				}
			}
			return
		}

		if addr, ok := stringField(lower, jsonAddressKeys); ok && addr != "" {
			importJSONHost(lower, addr, parentGroupID, res)
			return
		}

		// Not a recognized host or group container — treat it as a
		// transparent wrapper (e.g. Termius's top-level "vault" object)
		// and look for hosts/groups nested inside any of its values.
		descended := false
		for _, val := range v {
			switch val.(type) {
			case map[string]interface{}, []interface{}:
				walkJSON(val, parentGroupID, tmpStore, res)
				descended = true
			}
		}
		if !descended {
			res.Skipped = append(res.Skipped, "unrecognized JSON object (no host or group fields matched)")
		}
	}
}

func importJSONHost(lower map[string]interface{}, address, groupID string, res *Result) {
	if proto, ok := stringField(lower, jsonProtocolKeys); ok && proto != "" && !strings.EqualFold(proto, "ssh") {
		res.Skipped = append(res.Skipped, fmt.Sprintf("host %s: non-ssh protocol %q", address, proto))
		return
	}

	label, ok := stringField(lower, jsonLabelKeys)
	if !ok || label == "" {
		label = address
	}

	port := 22
	if p, ok := lower[matchKey(lower, jsonPortKeys)]; ok {
		switch pv := p.(type) {
		case float64:
			port = int(pv)
		case string:
			if v, err := strconv.Atoi(pv); err == nil && v > 0 {
				port = v
			}
		}
	}

	var tags []string
	if t, ok := lower[matchKey(lower, jsonTagsKeys)]; ok {
		if list, ok := t.([]interface{}); ok {
			for _, tag := range list {
				if s, ok := tag.(string); ok && s != "" {
					tags = append(tags, s)
				}
			}
		}
	}

	identity := model.Identity{Kind: model.IdentityAgent}
	if key, ok := stringField(lower, jsonKeyPathKeys); ok && key != "" {
		identity = model.Identity{Kind: model.IdentityKey, KeyPath: key}
	} else if pw, ok := stringField(lower, jsonPasswordKeys); ok && pw != "" {
		identity = model.Identity{Kind: model.IdentityPassword}
		res.Secrets[label] = pw
	}

	username, _ := stringField(lower, jsonUsernameKeys)

	res.Hosts = append(res.Hosts, model.Host{
		ID:       store.NewID(),
		Label:    label,
		Address:  address,
		Port:     port,
		Username: username,
		GroupID:  groupID,
		Tags:     tags,
		Identity: identity,
	})
}

func lowerKeyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

func matchKey(m map[string]interface{}, aliases []string) string {
	for _, a := range aliases {
		if _, ok := m[a]; ok {
			return a
		}
	}
	return ""
}

func stringField(m map[string]interface{}, aliases []string) (string, bool) {
	key := matchKey(m, aliases)
	if key == "" {
		return "", false
	}
	s, ok := m[key].(string)
	return s, ok
}
