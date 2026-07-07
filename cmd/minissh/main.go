// Command minissh is a Termius-compatible SSH host launcher.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danieluremix/minissh/internal/connect"
	"github.com/danieluremix/minissh/internal/importflow"
	"github.com/danieluremix/minissh/internal/model"
	"github.com/danieluremix/minissh/internal/sources"
	"github.com/danieluremix/minissh/internal/store"
	"github.com/danieluremix/minissh/internal/tui"
)

// version is set at build time via -ldflags; defaults to "dev" for local builds.
var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "minissh:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "minissh",
		Short:         "A keyboard-first SSH host manager for the terminal",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}
	root.AddCommand(importCmd(), importTermiusCmd(), lsCmd(), addCmd(), editCmd(), rmCmd(), connectCmd())
	return root
}

func runTUI() error {
	s, err := store.Load()
	if err != nil {
		return err
	}
	// Connecting to a host happens inside tui.Run itself (it suspends the
	// TUI for the ssh session via tea.ExecProcess and resumes afterward)
	// rather than exiting here to exec ssh — minissh stays open across
	// SSH sessions instead of quitting every time you connect.
	return tui.Run(s.Hosts, s.Groups)
}

func importCmd() *cobra.Command {
	var format string
	var includeSecrets bool
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import hosts from a Termius CSV/JSON export or an ssh_config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := format
			if id == "" {
				id = detectFormat(args[0])
			}
			src, ok := sources.ByID(id)
			if !ok || !src.RequiresFile {
				return fmt.Errorf("unknown --format %q (want csv, json, or sshconfig; for a live Termius install use `minissh import-termius`)", id)
			}

			res, err := src.Run(args[0])
			if err != nil {
				return err
			}

			keysDir, err := store.KeysDir()
			if err != nil {
				return err
			}
			summary, err := importflow.Persist(res, importflow.Options{
				KeysDir:      keysDir,
				StoreSecrets: includeSecrets,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Imported %d host(s), %d group(s).\n", summary.HostsImported, summary.GroupsImported)
			if len(res.Secrets) > 0 {
				if includeSecrets {
					fmt.Printf("Stored %d/%d password(s) in the OS keychain.\n", summary.PasswordsStored, len(res.Secrets))
				} else {
					fmt.Printf("%d host(s) use password auth; re-run with --include-secrets to store passwords in the OS keychain (otherwise ssh will just prompt).\n", len(res.Secrets))
				}
			}
			for _, reason := range res.Skipped {
				fmt.Println("skipped:", reason)
			}
			for _, w := range summary.Warnings {
				fmt.Println("warning:", w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "import format: csv, json, or sshconfig (default: guess from file extension)")
	cmd.Flags().BoolVar(&includeSecrets, "include-secrets", false, "store imported passwords in the OS keychain")
	return cmd
}

func detectFormat(path string) string {
	switch {
	case strings.HasSuffix(path, ".json"):
		return "json"
	case strings.HasSuffix(path, ".csv"):
		return "csv"
	default:
		return "sshconfig"
	}
}

func lsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known hosts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Load()
			if err != nil {
				return err
			}
			groupByID := make(map[string]model.Group, len(s.Groups))
			for _, g := range s.Groups {
				groupByID[g.ID] = g
			}
			for _, h := range s.Hosts {
				target := h.Address
				if h.Username != "" {
					target = h.Username + "@" + target
				}
				fmt.Printf("%-20s %-30s port %d\n", h.Label, target, h.Port)
			}
			return nil
		},
	}
}

func addCmd() *cobra.Command {
	var address, group, username, key string
	var port int
	cmd := &cobra.Command{
		Use:   "add <label>",
		Short: "Add a host manually",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]
			if address == "" {
				return fmt.Errorf("--address is required")
			}
			s, err := store.Load()
			if err != nil {
				return err
			}
			if store.FindHostIndex(s, label) != -1 {
				return fmt.Errorf("host %q already exists (use `minissh edit`)", label)
			}
			if port == 0 {
				port = 22
			}
			identity := model.Identity{Kind: model.IdentityAgent}
			if key != "" {
				identity = model.Identity{Kind: model.IdentityKey, KeyPath: key}
			}
			store.UpsertHost(s, model.Host{
				Label:    label,
				Address:  address,
				Port:     port,
				Username: username,
				GroupID:  store.UpsertGroup(s, group, ""),
				Identity: identity,
			})
			return store.Save(s)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "hostname or IP address (required)")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&username, "user", "", "SSH username")
	cmd.Flags().StringVar(&group, "group", "", "group name")
	cmd.Flags().StringVar(&key, "key", "", "path to private key")
	return cmd
}

func editCmd() *cobra.Command {
	var address, group, username, key string
	var port int
	cmd := &cobra.Command{
		Use:   "edit <label>",
		Short: "Edit an existing host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]
			s, err := store.Load()
			if err != nil {
				return err
			}
			idx := store.FindHostIndex(s, label)
			if idx == -1 {
				return fmt.Errorf("host %q not found", label)
			}
			h := &s.Hosts[idx]
			if cmd.Flags().Changed("address") {
				h.Address = address
			}
			if cmd.Flags().Changed("port") {
				h.Port = port
			}
			if cmd.Flags().Changed("user") {
				h.Username = username
			}
			if cmd.Flags().Changed("group") {
				h.GroupID = store.UpsertGroup(s, group, "")
			}
			if cmd.Flags().Changed("key") {
				h.Identity = model.Identity{Kind: model.IdentityKey, KeyPath: key}
			}
			return store.Save(s)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "hostname or IP address")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port")
	cmd.Flags().StringVar(&username, "user", "", "SSH username")
	cmd.Flags().StringVar(&group, "group", "", "group name")
	cmd.Flags().StringVar(&key, "key", "", "path to private key")
	return cmd
}

func rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <label>",
		Short: "Remove a host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Load()
			if err != nil {
				return err
			}
			if !store.DeleteHost(s, args[0]) {
				return fmt.Errorf("host %q not found", args[0])
			}
			return store.Save(s)
		},
	}
}

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <label>",
		Short: "Connect directly to a host by label, without the picker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Load()
			if err != nil {
				return err
			}
			idx := store.FindHostIndex(s, args[0])
			if idx == -1 {
				return fmt.Errorf("host %q not found", args[0])
			}
			return connect.Exec(s.Hosts[idx])
		},
	}
}
