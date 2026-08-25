package cmd

import (
	"fmt"

	"github.com/joeltjs/gstash/internal/git"

	"github.com/spf13/cobra"
)

var listAll bool

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List stashes (filtered by current branch by default)",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := workdir()
		entries, err := git.StashList(dir)
		if err != nil {
			return err
		}
		cur, _ := git.CurrentBranch(dir)
		if !listAll {
			entries = git.FilterByCurrent(entries, cur)
		}
		if len(entries) == 0 {
			fmt.Println("No stashes found.")
			return nil
		}
		scope := "current branch (" + orDash(cur) + ")"
		if listAll {
			scope = "all branches"
		}
		fmt.Printf("Stashes on %s:\n\n", scope)
		fmt.Printf("  %-4s %-19s %-15s %s\n", "ID", "BRANCH", "AGE", "MESSAGE")
		for _, e := range entries {
			src := ""
			switch e.Source {
			case git.SourceInferred:
				src = "~"
			case git.SourceUnknown:
				src = "?"
			}
			fmt.Printf("  #%-3d %-19s %-15s %s\n", e.Index, truncRunes(e.Branch+src, 19), truncRunes(e.Age, 15), e.Message)
		}
		fmt.Printf("\n%s\n", "~ = inferred from commits, ? = unknown")
		return nil
	},
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		if n <= 1 {
			return string(r[:n])
		}
		return string(r[:n-1]) + "…"
	}
	return s
}

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "show stashes for all branches")
	rootCmd.AddCommand(listCmd)
}
