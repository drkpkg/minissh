package importer

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/store"
)

// ImportSSHConfig parses an OpenSSH-style config file (~/.ssh/config), the
// format Termius itself can export to. Wildcard/pattern Host blocks (e.g.
// "Host *", "Host bastion-*") are skipped, since they're templates rather
// than concrete hosts.
func ImportSSHConfig(r io.Reader) (*Result, error) {
	res := &Result{Secrets: map[string]string{}}

	type block struct {
		alias        string
		hostname     string
		port         string
		user         string
		identityFile string
	}
	var blocks []block
	var cur *block

	flush := func() {
		if cur == nil {
			return
		}
		if cur.alias == "" || strings.ContainsAny(cur.alias, "*?") {
			if cur.alias != "" {
				res.Skipped = append(res.Skipped, "skipped wildcard Host pattern: "+cur.alias)
			}
			cur = nil
			return
		}
		blocks = append(blocks, *cur)
		cur = nil
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")

		switch key {
		case "host":
			flush()
			cur = &block{alias: fields[1]} // first pattern only; extra aliases on the line are ignored
		case "hostname":
			if cur != nil {
				cur.hostname = value
			}
		case "port":
			if cur != nil {
				cur.port = value
			}
		case "user":
			if cur != nil {
				cur.user = value
			}
		case "identityfile":
			if cur != nil {
				cur.identityFile = value
			}
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for _, b := range blocks {
		addr := b.hostname
		if addr == "" {
			addr = b.alias
		}
		port := 22
		if b.port != "" {
			if v, err := strconv.Atoi(b.port); err == nil && v > 0 {
				port = v
			}
		}
		identity := model.Identity{Kind: model.IdentityAgent}
		if b.identityFile != "" {
			identity = model.Identity{Kind: model.IdentityKey, KeyPath: expandHome(b.identityFile)}
		}
		res.Hosts = append(res.Hosts, model.Host{
			ID:       store.NewID(),
			Label:    b.alias,
			Address:  addr,
			Port:     port,
			Username: b.user,
			Identity: identity,
		})
	}

	return res, nil
}

func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return home + path[1:]
	}
	return path
}
