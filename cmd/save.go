package cmd

import (
	"fmt"

	"github.com/joeltjs/gstash/internal/git"

	"github.com/spf13/cobra"
)

var saveUntracked bool

var saveCmd = &cobra.Command{
	Use:   "save [message]",
	Short: "Create a stash (records the current branch in the message)",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		msg := "wip"
		if len(args) > 0 {
			msg = join(args)
		}
		out, err := git.Save(workdir(), msg, saveUntracked)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

func join(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

func init() {
	saveCmd.Flags().BoolVarP(&saveUntracked, "include-untracked", "u", false, "include untracked files")
	rootCmd.AddCommand(saveCmd)
}
