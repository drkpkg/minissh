# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

minissh is a keyboard-first terminal SSH host manager (Go, `charmbracelet/bubbletea` TUI) — a three-panel dashboard (groups / hosts / details) plus a full CLI, backed by a local JSON store. Module path: `github.com/drkpkg/minissh`.

## Commands

```sh
go build ./...
go vet ./...
go test ./...                     # add -race for CI parity
go test ./internal/store/...      # single package
go test ./internal/tui/ -run TestName   # single test
go build -o minissh ./cmd/minissh # produce the actual binary
golangci-lint run                 # see .golangci.yml — gosec, revive, errorlint enabled on top of standard
```

CI (`.github/workflows/ci.yml`) runs `go mod verify`, `go test -race -count=1 ./...`, a build, and `govulncheck`. A separate workflow runs golangci-lint. Match these locally before considering work done.

## Architecture

**Data flow:** `internal/model` defines the two core types, `Host` and `Group`, plus `Store` (`Hosts []Host` + `Groups []Group`). `internal/store` is the only package that reads/writes the persisted JSON file (`$XDG_CONFIG_HOME/minissh/hosts.json`, atomic write via temp file + rename). Both `cmd/minissh` (CLI) and `internal/tui` call `store.Load()`/`store.Save()` directly around each mutation — there is no in-process daemon or shared cache; every command/action re-reads and re-writes the whole file.

**Secrets never touch hosts.json.** Passwords and key passphrases live in the OS keychain (`internal/keychain`, via `zalando/go-keyring`) keyed by `Host.ID` (or `"key:"+label` for imported key passphrases). `model.Identity` only stores *how* to authenticate (agent/key/password) and a key file path — never a secret. Decrypted private keys from imports are written to `$XDG_CONFIG_HOME/minissh/keys/` as `0600` files.

**Connecting shells out to the real `ssh` binary** (`internal/connect`) rather than reimplementing the protocol — this is deliberate so `~/.ssh/config`, agent forwarding, and known_hosts all just work. There are three distinct ways a session gets started, each suited to a different caller:
- `connect.Exec` — `syscall.Exec`-replaces the current process. Used by `minissh connect <label>` (CLI passthrough).
- `connect.Command` + `tea.ExecProcess` — suspends the TUI, hands the terminal to a real ssh child process, and resumes the TUI when it exits. This is the `E` (full-screen) keybinding and the fallback if embedding fails.
- `internal/sshsession` — runs ssh in a pty (`creack/pty`) parsed by a `hinshun/vt10x` virtual terminal, rendered inline inside the HOSTS panel via lipgloss. This is what `enter` does normally: minissh stays running and the session renders in-place instead of taking over the whole screen. Note the unexported-bit-value dependency on vt10x documented at the top of `session.go`.

Both connect paths use `sshpass` transparently for password-auth hosts when a password is in the keychain and `sshpass` is on `PATH`; otherwise ssh prompts interactively as normal.

**Import pipeline** is a three-stage funnel so every source shares one persistence path: `internal/importer` parses an external format (CSV, JSON, `ssh_config`) or `internal/termius` decrypts a local Termius vault directly, both producing an `importer.Result` (hosts/groups/secrets/keys, format-agnostic). `internal/sources` is the registry (`sources.All`) mapping a source ID to its `Run` function — this is the single place to plug in a new import source; neither CLI nor TUI need to change. `internal/importflow.Persist` is the only place that actually writes an `importer.Result` into the store: it upserts groups/hosts, writes decrypted key files (`0600`), and stores secrets in the keychain — reviewed once here rather than duplicated per source.

**TUI structure** (`internal/tui`): `appModel` is a single bubbletea model driving the whole three-panel dashboard. Input routing is a priority chain checked in `Update`: an active embedded SSH session (`activeSession != nil`) captures all input first, then any open `overlay` (source picker / file prompt / import confirm / add-edit form / delete confirm), then live search (`searching`), then normal panel navigation (`updateMain`) dispatched by `focus` (groups/hosts/details). Each of those states owns its own `update*`/`*View` pair. `homeView` shows a dashboard (favorites/recent/stats) in the center panel until the user first interacts with the host table, then switches to the host table for the rest of the session. Mutations (add/edit/delete/import/favorite) call `store.Load()`, mutate, `store.Save()`, then `refreshFromStore` to update in-memory `allHosts`/`allGroups` and rebuild the groups/hosts panels — there's no reactive binding, views are rebuilt explicitly after each change.

**Reachability polling**: a `pollTick` timer periodically probes whichever hosts are currently visible in the hosts panel (not the whole inventory) with background TCP checks, feeding online/offline badges.

## Conventions worth knowing

- gosec's G204 (subprocess with variable input) is deliberately excluded globally — shelling out to `ssh`/`sshpass` with host-derived args is the app's core behavior, not a vulnerability here.
- gosec's G304 (file path from variable) is excluded specifically in `internal/sources`, `internal/store`, `internal/termius` — user-provided import paths and local config/vault reads are intentional there.
- Key/password handling: never persist a plaintext secret in `hosts.json`; always route through `internal/keychain`. When changing a host away from password auth, the orphaned keychain entry should be deleted (see `saveHostForm` in `internal/tui/app.go`).
