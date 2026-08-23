package cmd

import (
	"fmt"

	"gstash/internal/git"

	"github.com/spf13/cobra"
)

func singleIndexArg(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("requires exactly one stash index, e.g. gstash apply 0")
	}
	var n int
	if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil {
		return fmt.Errorf("invalid stash index: %s", args[0])
	}
	return nil
}

func indexFrom(args []string) int {
	n := 0
	fmt.Sscanf(args[0], "%d", &n)
	return n
}

var applyCmd = &cobra.Command{
	Use:   "apply <index>",
	Short: "Apply a stash without removing it",
	Args:  singleIndexArg,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := git.Apply(workdir(), stashRef(indexFrom(args)))
		fmt.Println(out)
		return err
	},
}

var popCmd = &cobra.Command{
	Use:   "pop <index>",
	Short: "Apply a stash and drop it",
	Args:  singleIndexArg,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := git.Pop(workdir(), stashRef(indexFrom(args)))
		fmt.Println(out)
		return err
	},
}

var dropForce bool

var dropCmd = &cobra.Command{
	Use:   "drop <index>",
	Short: "Delete a stash",
	Args:  singleIndexArg,
	RunE: func(cmd *cobra.Command, args []string) error {
		ref := stashRef(indexFrom(args))
		if !dropForce {
			fmt.Printf("Drop %s? [y/N] ", ref)
			var ans string
			fmt.Scanln(&ans)
			if ans != "y" && ans != "Y" && ans != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}
		out, err := git.Drop(workdir(), ref)
		fmt.Println(out)
		return err
	},
}

var branchName string

var branchCmd = &cobra.Command{
	Use:   "branch <index> [name]",
	Short: "Create a new branch from a stash and apply it there",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := branchName
		if len(args) == 2 {
			name = args[1]
		}
		if name == "" {
			name = fmt.Sprintf("stash-%s", args[0])
		}
		out, err := git.BranchFromStash(workdir(), stashRef(indexFrom(args)), name)
		fmt.Println(out)
		return err
	},
}

func init() {
	dropCmd.Flags().BoolVarP(&dropForce, "force", "y", false, "skip confirmation")
	branchCmd.Flags().StringVar(&branchName, "name", "", "branch name")
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(popCmd)
	rootCmd.AddCommand(dropCmd)
	rootCmd.AddCommand(branchCmd)
}
