package cli

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/alexandrealan/devnat/internal/agent"
)

func httpCmd() *cobra.Command {
	var (
		relayURL  string
		token     string
		subdomain string
		dashboard string
	)

	cmd := &cobra.Command{
		Use:   "http [host:]port",
		Short: "Expose a local HTTP service through the relay",
		Long: "Expose a local HTTP service through the relay.\n\n" +
			"Examples:\n" +
			"  devnat http 8080\n" +
			"  devnat http localhost:3000 --subdomain demo\n" +
			"  devnat http http://127.0.0.1:5000 --relay wss://devnat.example.com",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			local, err := normalizeLocal(args[0])
			if err != nil {
				return err
			}
			a, err := agent.New(agent.Config{
				Relay:     relayURL,
				Local:     local,
				Token:     token,
				Subdomain: subdomain,
				Dashboard: dashboard,
			})
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return a.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&relayURL, "relay", envOr("DEVNAT_RELAY", "wss://devnat.example.com"), "relay WebSocket URL")
	cmd.Flags().StringVar(&token, "token", os.Getenv("DEVNAT_TOKEN"), "auth token for the relay")
	cmd.Flags().StringVar(&subdomain, "subdomain", "", "requested subdomain (random if empty)")
	cmd.Flags().StringVar(&dashboard, "dashboard", "127.0.0.1:4040", "local dashboard address (empty to disable)")
	return cmd
}

// normalizeLocal accepts "8080", "localhost:3000", "127.0.0.1:5000" or a full
// URL and returns a normalized http(s) target.
func normalizeLocal(s string) (string, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s, nil
	}
	if _, err := strconv.Atoi(s); err == nil {
		return "http://127.0.0.1:" + s, nil
	}
	return "http://" + s, nil
}
