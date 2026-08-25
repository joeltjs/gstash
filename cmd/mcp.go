package cmd

import (
	"fmt"

	"github.com/joeltjs/gstash/internal/git"
	"github.com/joeltjs/gstash/internal/mcpserver"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the MCP server over stdio (for AI agents)",
	Long: `Expose gstash as Model Context Protocol tools so AI agents can list,
inspect, save, apply, pop, drop and branch git stashes without shell access.

Configure your agent with:
  {"command": "gstash", "args": ["mcp"]}
The working directory of that process must be inside the target repository.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := workdir()
		if !git.IsRepo(dir) {
			return fmt.Errorf("not a git repository")
		}
		return mcpserver.Run(dir)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
