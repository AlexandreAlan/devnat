package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/xtaci/smux"

	"github.com/alexandrealan/devnat/internal/tunnel"
)

// Config configures the relay gateway.
type Config struct {
	Addr   string // listen address, e.g. ":443" or ":8000"
	Domain string // public base domain, e.g. "devnat.example.com"
	Token  string // shared secret; empty disables auth
	Email  string // ACME contact email (used when TLS is enabled)
	Dev    bool   // dev mode: plain HTTP, no TLS
}

// Server is the public relay/gateway.
type Server struct {
	cfg      Config
	registry *registry
}

// New builds a relay server.
func New(cfg Config) *Server {
	return &Server{cfg: cfg, registry: newRegistry()}
}

// Run starts the relay and blocks until the context is cancelled or the
// listener fails.
func (s *Server) Run(ctx context.Context) error {
	if strings.TrimSpace(s.cfg.Domain) == "" {
		return errors.New("relay: --domain is required (e.g. devnat.example.com)")
	}

	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           http.HandlerFunc(s.handle),
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	scheme := "https"
	if s.cfg.Dev {
		scheme = "http"
	}
	log.Printf("relay: %s://*.%s listening on %s (dev=%v)", scheme, s.cfg.Domain, s.cfg.Addr, s.cfg.Dev)

	var err error
	if s.cfg.Dev {
		err = srv.ListenAndServe()
	} else {
		err = s.listenAndServeTLS(srv)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	sub := s.subdomain(hostname(r.Host))

	// Apex domain: agent registration endpoint or landing page.
	if sub == "" {
		if r.URL.Path == tunnel.TunnelPath {
			s.handleAgent(w, r)
			return
		}
		s.handleLanding(w, r)
		return
	}

	// Subdomain: visitor traffic for a tunnel.
	s.handleVisitor(w, r, sub)
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Agents are CLI clients, not browsers, so the same-origin check
		// does not apply.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	ctx := r.Context()

	var req tunnel.RegisterRequest
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	err = wsjson.Read(readCtx, c, &req)
	cancel()
	if err != nil {
		c.Close(websocket.StatusProtocolError, "register read failed")
		return
	}

	if s.cfg.Token != "" && req.Token != s.cfg.Token {
		_ = wsjson.Write(ctx, c, tunnel.RegisterResponse{OK: false, Error: "invalid token"})
		c.Close(websocket.StatusPolicyViolation, "invalid token")
		return
	}

	sub := normalizeSub(req.Subdomain)
	if sub == "" {
		sub = randomSub()
	}

	if !s.registry.reserve(sub) {
		_ = wsjson.Write(ctx, c, tunnel.RegisterResponse{OK: false, Error: "subdomain in use: " + sub})
		c.Close(websocket.StatusPolicyViolation, "subdomain in use")
		return
	}
	defer s.registry.remove(sub)

	// Confirm registration before the binary multiplexer takes over the
	// connection (the JSON handshake must complete first).
	if err := wsjson.Write(ctx, c, tunnel.RegisterResponse{
		OK:        true,
		URL:       s.publicURL(sub),
		Subdomain: sub,
	}); err != nil {
		c.Close(websocket.StatusInternalError, "register write failed")
		return
	}

	nc := tunnel.NetConn(context.Background(), c)
	sess, err := smux.Client(nc, tunnel.SmuxConfig())
	if err != nil {
		c.Close(websocket.StatusInternalError, "mux init failed")
		return
	}
	defer sess.Close()
	s.registry.bind(sub, sess)

	log.Printf("relay: tunnel opened  %s", s.publicURL(sub))
	defer log.Printf("relay: tunnel closed  %s", s.publicURL(sub))

	// The agent does not open streams toward the relay; AcceptStream blocks
	// until the session dies, which is our signal to clean up.
	for {
		if _, err := sess.AcceptStream(); err != nil {
			return
		}
	}
}

func (s *Server) handleVisitor(w http.ResponseWriter, r *http.Request, sub string) {
	sess, ok := s.registry.session(sub)
	if !ok {
		http.Error(w, fmt.Sprintf("devnat: no active tunnel for %q", sub), http.StatusNotFound)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "tunnel" // ignored: the dialer returns a mux stream
			if req.Header.Get("X-Forwarded-Proto") == "" {
				proto := "https"
				if s.cfg.Dev {
					proto = "http"
				}
				req.Header.Set("X-Forwarded-Proto", proto)
			}
		},
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return sess.OpenStream()
			},
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "devnat: tunnel error: "+err.Error(), http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "DevNAT relay online — %d active tunnel(s).\n", s.registry.count())
}

func (s *Server) publicURL(sub string) string {
	scheme := "https"
	if s.cfg.Dev {
		scheme = "http"
	}
	hostport := s.cfg.Domain
	if _, port, err := net.SplitHostPort(s.cfg.Addr); err == nil && port != "" {
		if (s.cfg.Dev && port != "80") || (!s.cfg.Dev && port != "443") {
			hostport = s.cfg.Domain + ":" + port
		}
	}
	return fmt.Sprintf("%s://%s.%s", scheme, sub, hostport)
}

func (s *Server) subdomain(host string) string {
	if host == s.cfg.Domain {
		return ""
	}
	suffix := "." + s.cfg.Domain
	if strings.HasSuffix(host, suffix) {
		return strings.TrimSuffix(host, suffix)
	}
	return ""
}

func hostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func randomSub() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var reservedSubs = map[string]struct{}{
	"www": {}, "tunnel": {}, "relay": {}, "api": {}, "admin": {}, "app": {},
}

func normalizeSub(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = out[:40]
	}
	if _, reserved := reservedSubs[out]; reserved {
		return ""
	}
	return out
}
