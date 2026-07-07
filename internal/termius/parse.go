package termius

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/danieluremix/minissh/internal/importer"
	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/store"
)

// parsedData is the intermediate, loosely-typed representation of every
// decrypted JSON blob, classified by shape (Termius's real internal schema
// isn't documented, so this mirrors the reference script's heuristics).
type parsedData struct {
	identities  []map[string]interface{} // objects with both "username" and "password"
	keysByLabel map[string]map[string]interface{}
	keysByID    map[string]map[string]interface{}
	connections []map[string]interface{} // objects with "host", "user_name", "connection_type"
}

func parseBlocks(blocks [][]byte) parsedData {
	pd := parsedData{
		keysByLabel: map[string]map[string]interface{}{},
		keysByID:    map[string]map[string]interface{}{},
	}
	for _, b := range blocks {
		trimmed := bytes.TrimSpace(b)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			continue
		}

		if _, hasUser := obj["username"]; hasUser {
			if _, hasPass := obj["password"]; hasPass {
				pd.identities = append(pd.identities, obj)
			}
		}

		if pk, ok := obj["private_key"].(string); ok && pk != "" {
			if label, ok := obj["label"].(string); ok && label != "" {
				pd.keysByLabel[label] = obj
				if id, ok := obj["id"]; ok {
					pd.keysByID[fmt.Sprint(id)] = obj
				}
			}
		}

		if host, ok := obj["host"].(string); ok && host != "" {
			if _, ok := obj["user_name"]; ok {
				if _, ok := obj["connection_type"]; ok {
					pd.connections = append(pd.connections, obj)
				}
			}
		}
	}
	return pd
}

// build joins parsed connections/identities/keys into a Result.
//
// Two bugs present in the reference script are fixed here:
//   - password matching now takes the *first* identity found for a given
//     username (deterministic), not whichever happens to be last in
//     iteration order;
//   - host dedup keys on host:port:username instead of just host:port, so
//     a second connection reusing the same host:port with a different user
//     isn't silently dropped.
func build(pd parsedData) *importer.Result {
	res := &importer.Result{Secrets: map[string]string{}, Keys: map[string]importer.KeyMaterial{}}

	passwordByUsername := map[string]string{}
	for _, obj := range pd.identities {
		username, _ := obj["username"].(string)
		password, _ := obj["password"].(string)
		if username == "" || password == "" {
			continue
		}
		if _, exists := passwordByUsername[username]; !exists {
			passwordByUsername[username] = password
		}
	}

	for label, obj := range pd.keysByLabel {
		pk, _ := obj["private_key"].(string)
		passphrase, _ := obj["passphrase"].(string)
		res.Keys[label] = importer.KeyMaterial{PrivateKeyPEM: pk, Passphrase: passphrase}
	}

	seen := map[string]bool{}
	for _, conn := range pd.connections {
		host, _ := conn["host"].(string)
		userName, _ := conn["user_name"].(string)
		connType, _ := conn["connection_type"].(string)
		if host == "" {
			continue
		}
		if connType != "" && !strings.EqualFold(connType, "ssh") {
			res.Skipped = append(res.Skipped, fmt.Sprintf("host %s: non-ssh connection_type %q", host, connType))
			continue
		}

		port := 22
		switch p := conn["port"].(type) {
		case float64:
			if int(p) > 0 {
				port = int(p)
			}
		case string:
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				port = v
			}
		}

		dedupKey := fmt.Sprintf("%s:%d:%s", host, port, userName)
		if seen[dedupKey] {
			res.Skipped = append(res.Skipped, fmt.Sprintf("duplicate connection skipped: %s", dedupKey))
			continue
		}
		seen[dedupKey] = true

		label, _ := conn["title"].(string)
		if label == "" {
			label = host
		}

		identity := model.Identity{Kind: model.IdentityAgent}
		if keyIDRaw, ok := conn["key_id"]; ok {
			if keyObj, ok := pd.keysByID[fmt.Sprint(keyIDRaw)]; ok {
				if keyLabel, ok := keyObj["label"].(string); ok && keyLabel != "" {
					// Placeholder: KeyPath holds the key's *label*, not a
					// real path yet. The caller resolves it against
					// Result.Keys and writes the file before persisting.
					identity = model.Identity{Kind: model.IdentityKey, KeyPath: keyLabel}
				}
			}
		} else if pw, ok := passwordByUsername[userName]; ok && pw != "" {
			identity = model.Identity{Kind: model.IdentityPassword}
			res.Secrets[label] = pw
		}

		res.Hosts = append(res.Hosts, model.Host{
			ID:       store.NewID(),
			Label:    label,
			Address:  host,
			Port:     port,
			Username: userName,
			Identity: identity,
		})
	}

	return res
}
