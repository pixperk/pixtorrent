package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pixtorrent",
	Short: "A minimalistic BitTorrent client",
	Long: `pixTorrent - A lightweight BitTorrent implementation in Go

Features:
  - Piece hash verification for data integrity
  - Tit-for-tat choking/unchoking algorithm
  - Rarest-first piece selection
  - Redis-backed tracker`,
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
