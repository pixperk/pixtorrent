package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pixtorrent",
	Short: "A minimalistic BitTorrent client",
	Long:  Cyan + Bold + logoSmall + Reset + "\n  " + Dim + "A lightweight BitTorrent implementation in Go" + Reset,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
