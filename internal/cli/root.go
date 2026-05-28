package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/alexandrealan/devnat/internal/buildinfo"
)

// Execute runs the devnat command-line interface.
func Execute() error {
	root := &cobra.Command{
		Use:           "devnat",
		Short:         "DevNAT — expose a local service to the internet over an outbound 443 tunnel",
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(httpCmd(), relayCmd())
	return root.Execute()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
