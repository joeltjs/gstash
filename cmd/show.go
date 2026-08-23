package cmd

import (
	"fmt"

	"gstash/internal/git"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:     "show [index]",
	Aliases: []string{"diff"},
	Short:   "Show the diff of a stash",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n := 0
		if len(args) == 1 {
			fmt.Sscanf(args[0], "%d", &n)
		}
		out, err := git.Show(workdir(), stashRef(n))
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
