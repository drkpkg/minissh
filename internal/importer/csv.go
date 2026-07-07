package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/store"
)

// csvColumnAliases maps our canonical field name to the header names Termius
// (and similar tools) are known to use for it. Matching is case-insensitive
// and ignores surrounding whitespace.
var csvColumnAliases = map[string][]string{
	"label":    {"label", "name", "host alias", "alias", "title"},
	"address":  {"address", "hostname", "host", "ip address", "ip"},
	"port":     {"port"},
	"protocol": {"protocol"},
	"group":    {"group"},
	"subgroup": {"subgroup", "sub-group", "sub group"},
	"tags":     {"tags"},
	"username": {"username", "user"},
	"password": {"password"},
	"key":      {"key", "identity", "identity file", "ssh key", "keypath"},
}

// ImportCSV parses a Termius-style CSV export (label, address, protocol,
// port, group, subgroup, tags, username, password/key). Unknown columns are
// ignored. Rows with a non-ssh protocol are skipped, since minissh is
// SSH-only. Passwords are returned in Result.Secrets rather than on the
// Host, so the caller can decide whether to persist them to the keychain.
func ImportCSV(r io.Reader) (*Result, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		if err == io.EOF {
			return &Result{}, nil
		}
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	colIdx := indexColumns(header)

	res := &Result{Secrets: map[string]string{}}
	tmpStore := &model.Store{}

	rowNum := 1
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV row %d: %w", rowNum, err)
		}
		rowNum++

		get := func(field string) string {
			idx, ok := colIdx[field]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}

		label := get("label")
		address := get("address")
		if label == "" && address == "" {
			continue // blank row
		}
		if label == "" {
			label = address
		}
		if address == "" {
			res.Skipped = append(res.Skipped, fmt.Sprintf("row %d (%s): missing address", rowNum, label))
			continue
		}

		if proto := get("protocol"); proto != "" && !strings.EqualFold(proto, "ssh") {
			res.Skipped = append(res.Skipped, fmt.Sprintf("row %d (%s): non-ssh protocol %q", rowNum, label, proto))
			continue
		}

		port := 22
		if p := get("port"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				port = v
			}
		}

		groupID := ""
		if g := get("group"); g != "" {
			groupID = store.UpsertGroup(tmpStore, g, "")
			if sg := get("subgroup"); sg != "" {
				groupID = store.UpsertGroup(tmpStore, sg, groupID)
			}
		}

		var tags []string
		if t := get("tags"); t != "" {
			for _, tag := range strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == ';' }) {
				if tag = strings.TrimSpace(tag); tag != "" {
					tags = append(tags, tag)
				}
			}
		}

		identity := model.Identity{Kind: model.IdentityAgent}
		if key := get("key"); key != "" {
			identity = model.Identity{Kind: model.IdentityKey, KeyPath: key}
		} else if pw := get("password"); pw != "" {
			identity = model.Identity{Kind: model.IdentityPassword}
			res.Secrets[label] = pw
		}

		res.Hosts = append(res.Hosts, model.Host{
			ID:       store.NewID(),
			Label:    label,
			Address:  address,
			Port:     port,
			Username: get("username"),
			GroupID:  groupID,
			Tags:     tags,
			Identity: identity,
		})
	}

	res.Groups = tmpStore.Groups
	return res, nil
}

func indexColumns(header []string) map[string]int {
	byHeader := make(map[string]int, len(header))
	for i, h := range header {
		byHeader[strings.ToLower(strings.TrimSpace(h))] = i
	}
	idx := make(map[string]int, len(csvColumnAliases))
	for field, aliases := range csvColumnAliases {
		for _, alias := range aliases {
			if i, ok := byHeader[alias]; ok {
				idx[field] = i
				break
			}
		}
	}
	return idx
}
