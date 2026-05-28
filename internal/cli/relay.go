package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/alexandrealan/devnat/internal/relay"
)

func relayCmd() *cobra.Command {
	var cfg relay.Config

	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Run the public relay/gateway",
		Long: "Run the public relay/gateway.\n\n" +
			"Production (auto HTTPS, needs *.domain DNS -> this host):\n" +
			"  devnat relay --domain devnat.example.com --email you@example.com --token <secret>\n\n" +
			"Local development (plain HTTP):\n" +
			"  devnat relay --dev --domain localhost --addr :8000",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return relay.New(cfg).Run(ctx)
		},
	}

	cmd.Flags().StringVar(&cfg.Addr, "addr", envOr("DEVNAT_ADDR", ":443"), "listen address")
	cmd.Flags().StringVar(&cfg.Domain, "domain", os.Getenv("DEVNAT_DOMAIN"), "public base domain (e.g. devnat.example.com)")
	cmd.Flags().StringVar(&cfg.Token, "token", os.Getenv("DEVNAT_TOKEN"), "shared auth token (empty disables auth)")
	cmd.Flags().StringVar(&cfg.Email, "email", os.Getenv("DEVNAT_EMAIL"), "ACME contact email for automatic TLS")
	cmd.Flags().BoolVar(&cfg.Dev, "dev", false, "dev mode: plain HTTP, no TLS")
	return cmd
}
