package importer

import (
	"strings"
	"testing"

	"github.com/drkpkg/minissh/internal/model"
)

func TestImportJSONFlatArray(t *testing.T) {
	input := `[
		{"label": "web-1", "address": "10.0.0.1", "port": 22, "username": "root"},
		{"name": "db-1", "hostname": "10.0.0.2", "password": "hunter2"}
	]`
	res, err := ImportJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if len(res.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d: %+v", len(res.Hosts), res.Hosts)
	}
	if res.Hosts[1].Identity.Kind != model.IdentityPassword || res.Secrets["db-1"] != "hunter2" {
		t.Fatalf("expected db-1 password captured, got %+v secrets=%v", res.Hosts[1], res.Secrets)
	}
}

func TestImportJSONNestedVaultGroupsHosts(t *testing.T) {
	input := `{
		"vault": {
			"groups": [
				{
					"name": "prod",
					"hosts": [{"label": "web-1", "address": "10.0.0.1"}],
					"groups": [
						{"name": "db", "hosts": [{"label": "db-1", "address": "10.0.0.2", "port": 5432}]}
					]
				}
			]
		}
	}`
	res, err := ImportJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if len(res.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d: %+v", len(res.Hosts), res.Hosts)
	}
	if len(res.Groups) != 2 {
		t.Fatalf("expected 2 groups (prod, db), got %d: %+v", len(res.Groups), res.Groups)
	}

	byLabel := map[string]model.Host{}
	for _, h := range res.Hosts {
		byLabel[h.Label] = h
	}
	web1, db1 := byLabel["web-1"], byLabel["db-1"]
	if web1.GroupID == "" || db1.GroupID == "" {
		t.Fatalf("expected both hosts to have a group, got web1=%+v db1=%+v", web1, db1)
	}
	if web1.GroupID == db1.GroupID {
		t.Fatal("expected web-1 (prod) and db-1 (prod/db) to be in different groups")
	}

	var prodGroup, dbGroup model.Group
	for _, g := range res.Groups {
		if g.Name == "prod" {
			prodGroup = g
		}
		if g.Name == "db" {
			dbGroup = g
		}
	}
	if dbGroup.ParentID != prodGroup.ID {
		t.Fatalf("expected db group's parent to be prod group, got %+v / %+v", dbGroup, prodGroup)
	}
	if db1.GroupID != dbGroup.ID {
		t.Fatalf("expected db-1 host to belong to db group, got %+v", db1)
	}
}

func TestImportJSONSkipsUnrecognizedAndNonSSH(t *testing.T) {
	input := `{
		"hosts": [
			{"label": "web-1", "address": "10.0.0.1", "protocol": "telnet"},
			{"label": "no-address-here"},
			{"label": "web-2", "address": "10.0.0.2"}
		]
	}`
	res, err := ImportJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if len(res.Hosts) != 1 || res.Hosts[0].Label != "web-2" {
		t.Fatalf("expected only web-2 imported, got %+v", res.Hosts)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("expected 2 skipped entries, got %d: %v", len(res.Skipped), res.Skipped)
	}
}

func TestImportJSONEmpty(t *testing.T) {
	res, err := ImportJSON(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ImportJSON empty: %v", err)
	}
	if len(res.Hosts) != 0 {
		t.Fatalf("expected no hosts, got %d", len(res.Hosts))
	}
}
