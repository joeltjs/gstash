package cmd

import (
	"fmt"
	"os"

	"gstash/internal/git"
	"gstash/internal/tui"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gstash",
	Short: "Git stash manager with a visual terminal UI",
	Long: `gstash lets you browse, preview, apply, pop and drop git stashes,
filtered by the current branch (ala GitKraken, in your terminal).`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		if !git.IsRepo(dir) {
			return fmt.Errorf("not a git repository")
		}
		return tui.Run(dir)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func workdir() string {
	dir, err := os.Getwd()
	cobra.CheckErr(err)
	if !git.IsRepo(dir) {
		cobra.CheckErr(fmt.Errorf("not a git repository"))
	}
	return dir
}

func stashRef(n int) string {
	return fmt.Sprintf("stash@{%d}", n)
}
