# minissh

A fast, keyboard-first SSH host manager for the terminal — a three-panel
dashboard (groups / hosts / details) in the style of
[LazyGit](https://github.com/jesseduffield/lazygit),
[k9s](https://github.com/derailed/k9s), [GitUI](https://github.com/extrawurst/gitui),
and [btop](https://github.com/aristocratos/btop).

![CI](https://github.com/drkpkg/minissh/actions/workflows/ci.yml/badge.svg)

## Features

- **Three-panel dashboard** — collapsible group tree, sortable host table, live
  details panel, all on screen at once.
- **Single-key navigation** — `h`/`l`/`tab` between panels, `j`/`k` to move,
  `enter` to connect, no nested menus for common actions.
- **Sessions render inline.** Pressing `enter` opens a real terminal emulator
  (pty + VT100 parser) embedded in the host panel — minissh stays open across
  SSH sessions instead of quitting every time you connect. A full-screen
  fallback (`E`) is always available too.
- **Live fuzzy search** across hostname, address, user, tags, and group.
- **Favorites, last-connected tracking, and per-host notes.**
- **Live reachability polling** — online/offline badges from background TCP
  probes of whatever's currently visible, not your whole inventory.
- **Agent, SSH key, or password authentication.** Passwords and key
  passphrases are stored in your OS keychain — never written to disk in
  plaintext.
- **Import hosts** from a CSV file, a JSON file, an `ssh_config` file, or by
  decrypting a local Termius installation's vault directly (useful for a
  one-time migration).
- **Everything is also a scriptable CLI subcommand** — add, edit, remove,
  list, import, and connect to hosts without opening the TUI.

## Installation

```sh
go install github.com/drkpkg/minissh/cmd/minissh@latest
```

Or download a prebuilt binary from the
[releases page](https://github.com/drkpkg/minissh/releases). Linux builds are
available for x86_64 (`amd64`), ARM64 (`arm64`), and 32-bit ARMv7 (`armv7`,
e.g. Raspberry Pi 2+ on a 32-bit OS).

### Debian / Ubuntu

Download the `.deb` for your architecture from the
[releases page](https://github.com/drkpkg/minissh/releases) and install it:

```sh
sudo apt install ./minissh_<version>_amd64.deb
```

### Arch Linux

Download the `.pkg.tar.zst` for your architecture from the
[releases page](https://github.com/drkpkg/minissh/releases) and install it:

```sh
sudo pacman -U minissh-<version>-1-x86_64.pkg.tar.zst
```

Or build from source with the PKGBUILD in `packaging/aur/`:

```sh
git clone https://github.com/drkpkg/minissh.git
cd minissh/packaging/aur
makepkg -si
```

Requires the system `ssh` client to be installed and on your `PATH` — minissh
shells out to it rather than reimplementing the SSH protocol, so your
existing `~/.ssh/config`, agent, and known_hosts all just work.

## Quick start

```sh
minissh                       # launch the dashboard
```

On first run, with no hosts yet, you'll see an onboarding screen. Press `a`
to add a host manually, or `i` to import from a file or a local Termius
install. Once you have hosts, `enter` on the selected one connects — the
session renders right there in the host panel, and you're back at the
dashboard the moment it ends.

## Keybindings

| Key | Action |
| --- | --- |
| `h` / `l` or `tab` / `shift+tab` | Move focus between panels |
| `j` / `k` | Move up/down within the focused panel |
| `enter` | Connect (on the host panel) / drill into that group's hosts (on groups) |
| `E` | Connect full-screen instead of inline |
| `space` | Toggle favorite (hosts) · expand/collapse (groups) |
| `s` | Cycle the host table's sort column |
| `a` | Add a host |
| `e` | Edit the selected host |
| `d` | Delete the selected host |
| `i` | Import hosts |
| `/` | Live fuzzy search (`tab` cycles which field it matches) |
| `q` / `ctrl+c` | Quit |

The status bar at the bottom always shows the keys relevant to whatever
panel currently has focus.

## CLI reference

| Command | Description |
| --- | --- |
| `minissh` | Launch the dashboard |
| `minissh add <label> --address <addr> [--port] [--user] [--group] [--key]` | Add a host |
| `minissh edit <label> [--address] [--port] [--user] [--group] [--key]` | Edit an existing host |
| `minissh rm <label>` | Remove a host |
| `minissh ls` | List all known hosts |
| `minissh connect <label>` | Connect directly by label, without the picker |
| `minissh import <file> [--format csv\|json\|sshconfig] [--include-secrets]` | Import hosts from a file |
| `minissh import-termius` | Decrypt and import hosts from a local Termius installation |
| `minissh --version` | Print the build version |

## Data storage

- Hosts and groups: `$XDG_CONFIG_HOME/minissh/hosts.json` (`~/.config/minissh/hosts.json` on Linux)
- Imported/decrypted private keys: `$XDG_CONFIG_HOME/minissh/keys/` (written `0600`, directory `0700`)
- Passwords and key passphrases: your OS keychain (Secret Service/GNOME
  Keyring on Linux, Keychain on macOS, Credential Manager on Windows) — never
  in `hosts.json`

## Building from source

```sh
git clone https://github.com/drkpkg/minissh.git
cd minissh
go build -o minissh ./cmd/minissh
```

### Development

```sh
go build ./...
go vet ./...
go test ./...
```
