// Package sources is the registry of import sources minissh knows how to
// run — the single place a new source (another SSH client's export format,
// or another way of reaching into a local vault) gets plugged in. Neither
// the CLI nor the TUI need to change to pick up a new entry here.
package sources

import (
	"io"
	"os"

	"github.com/drkpkg/minissh/internal/importer"
	"github.com/drkpkg/minissh/internal/termius"
)

// Source describes one importable source of hosts.
type Source struct {
	ID           string
	Name         string
	Description  string
	RequiresFile bool
	// Run executes the import. filePath is ignored when RequiresFile is false.
	Run func(filePath string) (*importer.Result, error)
}

// All is the registry of every import source minissh supports.
var All = []Source{
	{
		ID:           "csv",
		Name:         "CSV file",
		Description:  "Termius's own CSV export (Settings → Export)",
		RequiresFile: true,
		Run:          runFile(importer.ImportCSV),
	},
	{
		ID:           "json",
		Name:         "JSON file",
		Description:  "Termius JSON export (best-effort, schema undocumented)",
		RequiresFile: true,
		Run:          runFile(importer.ImportJSON),
	},
	{
		ID:           "sshconfig",
		Name:         "ssh_config file",
		Description:  "An OpenSSH-style config file (~/.ssh/config)",
		RequiresFile: true,
		Run:          runFile(importer.ImportSSHConfig),
	},
	{
		ID:           "termius-live",
		Name:         "Termius (local vault)",
		Description:  "Decrypt hosts directly from a local Termius installation",
		RequiresFile: false,
		Run:          func(string) (*importer.Result, error) { return termius.Import() },
	},
}

// ByID looks up a registered source by ID.
func ByID(id string) (Source, bool) {
	for _, s := range All {
		if s.ID == id {
			return s, true
		}
	}
	return Source{}, false
}

// runFile adapts a parser that takes an io.Reader into a Source.Run that
// takes a file path.
func runFile(parse func(io.Reader) (*importer.Result, error)) func(string) (*importer.Result, error) {
	return func(path string) (*importer.Result, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		return parse(f)
	}
}
