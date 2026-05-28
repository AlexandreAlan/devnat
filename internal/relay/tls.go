package relay

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/caddyserver/certmagic"
)

// listenAndServeTLS serves the relay over HTTPS, issuing per-subdomain
// certificates on demand via the TLS-ALPN-01 challenge. This only requires a
// wildcard DNS record (*.domain -> relay IP); no DNS provider credentials are
// needed.
func (s *Server) listenAndServeTLS(srv *http.Server) error {
	if dir := os.Getenv("DEVNAT_CERT_DIR"); dir != "" {
		certmagic.Default.Storage = &certmagic.FileStorage{Path: dir}
	}

	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = s.cfg.Email

	cfg := certmagic.NewDefault()
	cfg.OnDemand = &certmagic.OnDemandConfig{
		DecisionFunc: func(ctx context.Context, name string) error {
			if name == s.cfg.Domain || strings.HasSuffix(name, "."+s.cfg.Domain) {
				return nil
			}
			return fmt.Errorf("name %q not served by this relay", name)
		},
	}

	tlsConfig := cfg.TLSConfig()
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	ln, err := tls.Listen("tcp", srv.Addr, tlsConfig)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}
