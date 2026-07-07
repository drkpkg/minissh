package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/danieluremix/minissh/internal/importflow"
	"github.com/danieluremix/minissh/internal/sources"
	"github.com/danieluremix/minissh/internal/store"
)

func importTermiusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-termius",
		Short: "Decrypt and import hosts directly from a local Termius installation",
		Long: "Locates the local Termius app's vault, decrypts it using the local\n" +
			"encryption key Termius itself stores in the OS keychain, and imports\n" +
			"the hosts it can reconstruct. Use this when Termius's own export\n" +
			"feature isn't available. Only reads your own local files and your own\n" +
			"OS keychain entry — nothing is sent anywhere, and nothing is written\n" +
			"until you confirm.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, _ := sources.ByID("termius-live")
			res, err := src.Run("")
			if err != nil {
				return err
			}

			if res.Stats != nil {
				fmt.Printf("Scanned local Termius vault: %d encrypted block(s) found, %d decrypted.\n", res.Stats.BlocksFound, res.Stats.BlocksDecrypted)
			}
			if len(res.Hosts) == 0 {
				fmt.Println("No hosts recovered.")
				for _, reason := range res.Skipped {
					fmt.Println("skipped:", reason)
				}
				return nil
			}

			fmt.Printf("\nFound %d host(s):\n", len(res.Hosts))
			for _, h := range res.Hosts {
				target := h.Address
				if h.Username != "" {
					target = h.Username + "@" + target
				}
				fmt.Printf("  %-20s %-30s port %d (%s)\n", h.Label, target, h.Port, h.Identity.Kind)
			}
			if len(res.Skipped) > 0 {
				fmt.Println()
				for _, reason := range res.Skipped {
					fmt.Println("skipped:", reason)
				}
			}

			keysDir, err := store.KeysDir()
			if err != nil {
				return err
			}
			fmt.Printf("\nImport %d host(s) into minissh? Passwords go straight to the OS keychain; private keys are written to %s (0600). Nothing else touches disk. [y/N] ", len(res.Hosts), keysDir)
			if !confirm() {
				fmt.Println("Aborted, nothing written.")
				return nil
			}

			summary, err := importflow.Persist(res, importflow.Options{KeysDir: keysDir, StoreSecrets: true})
			if err != nil {
				return err
			}
			fmt.Printf("Imported %d host(s): %d password(s) stored in the OS keychain, %d key file(s) written to %s.\n",
				summary.HostsImported, summary.PasswordsStored, summary.KeysWritten, keysDir)
			for _, w := range summary.Warnings {
				fmt.Println("warning:", w)
			}
			return nil
		},
	}
}

func confirm() bool {
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
