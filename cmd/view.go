package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joeltjs/gstash/internal/web"

	"github.com/spf13/cobra"
)

var viewPort int

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Open the web dashboard to browse stashes with Accept/Reject",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := workdir()
		port, err := resolvePort(cmd, dir)
		if err != nil {
			return err
		}
		addr, err := web.Serve(dir, port)
		if err != nil {
			return err
		}
		fmt.Printf("Dashboard running at http://%s (Ctrl+C to stop)\n", addr)
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		fmt.Println("\nBye.")
		return nil
	},
}

func resolvePort(cmd *cobra.Command, dir string) (int, error) {
	if cmd.Flags().Changed("port") {
		return viewPort, nil
	}
	return web.ResolvePort(dir)
}

func init() {
	viewCmd.Flags().IntVar(&viewPort, "port", 0, "dashboard port (defaults to GSTASH_DASHBOARD_PORT in .env, or fallback)")
	rootCmd.AddCommand(viewCmd)
}
