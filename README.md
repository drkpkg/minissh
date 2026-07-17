# minissh

A fast, keyboard-first SSH host manager for the terminal.

<img width="2376" height="1289" alt="image" src="https://github.com/user-attachments/assets/0a9b3d0c-d270-4761-8f21-e7a76b413254" />


![CI](https://github.com/drkpkg/minissh/actions/workflows/ci.yml/badge.svg)

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
