package agent

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/xtaci/smux"

	"github.com/alexandrealan/devnat/internal/tunnel"
)

// Config configures the agent (tunnel client).
type Config struct {
	Relay     string // relay base URL, e.g. wss://devnat.example.com
	Local     string // local target, e.g. http://127.0.0.1:8080
	Token     string
	Subdomain string // requested subdomain (random if empty)
	Dashboard string // local dashboard address; empty disables it
}

// Agent forwards traffic from the relay to a local service.
type Agent struct {
	cfg      Config
	localURL *url.URL
	insp     *inspector
}

// New validates the configuration and builds an agent.
func New(cfg Config) (*Agent, error) {
	lu, err := url.Parse(cfg.Local)
	if err != nil || lu.Host == "" {
		return nil, fmt.Errorf("invalid local target %q", cfg.Local)
	}
	return &Agent{cfg: cfg, localURL: lu, insp: newInspector(250)}, nil
}

// Run connects to the relay and serves tunnelled requests until the context
// is cancelled or the connection drops.
func (a *Agent) Run(ctx context.Context) error {
	if a.cfg.Dashboard != "" {
		go a.serveDashboard()
	}

	wsURL := strings.TrimRight(a.cfg.Relay, "/") + tunnel.TunnelPath
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("connect relay: %w", err)
	}

	if err := wsjson.Write(ctx, c, tunnel.RegisterRequest{
		Token:     a.cfg.Token,
		Subdomain: a.cfg.Subdomain,
	}); err != nil {
		c.Close(websocket.StatusInternalError, "register write")
		return fmt.Errorf("register: %w", err)
	}

	var resp tunnel.RegisterResponse
	if err := wsjson.Read(ctx, c, &resp); err != nil {
		c.Close(websocket.StatusInternalError, "register read")
		return fmt.Errorf("register response: %w", err)
	}
	if !resp.OK {
		c.Close(websocket.StatusNormalClosure, "rejected")
		return fmt.Errorf("relay rejected tunnel: %s", resp.Error)
	}

	log.Printf("tunnel up   %s  ->  %s", resp.URL, a.cfg.Local)
	if a.cfg.Dashboard != "" {
		log.Printf("dashboard   http://%s", a.cfg.Dashboard)
	}

	nc := tunnel.NetConn(context.Background(), c)
	sess, err := smux.Server(nc, tunnel.SmuxConfig())
	if err != nil {
		c.Close(websocket.StatusInternalError, "mux init")
		return fmt.Errorf("mux: %w", err)
	}
	defer sess.Close()

	go func() {
		<-ctx.Done()
		sess.Close()
	}()

	srv := &http.Server{Handler: a.proxyHandler()}
	err = srv.Serve(&smuxListener{sess: sess})
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func (a *Agent) proxyHandler() http.Handler {
	rp := httputil.NewSingleHostReverseProxy(a.localURL)
	director := rp.Director
	rp.Director = func(req *http.Request) {
		director(req)
		req.Host = a.localURL.Host
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "devnat: local service unreachable at %s: %v", a.cfg.Local, err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		rp.ServeHTTP(sw, r)
		a.insp.record(reqRecord{
			Time:       start,
			Method:     r.Method,
			Path:       r.URL.RequestURI(),
			Status:     sw.status,
			DurationMs: time.Since(start).Milliseconds(),
		})
	})
}

// smuxListener turns a multiplexer session into a net.Listener: each accepted
// stream is one inbound connection that http.Server serves.
type smuxListener struct{ sess *smux.Session }

func (l *smuxListener) Accept() (net.Conn, error) { return l.sess.AcceptStream() }
func (l *smuxListener) Close() error              { return l.sess.Close() }
func (l *smuxListener) Addr() net.Addr            { return muxAddr{} }

type muxAddr struct{}

func (muxAddr) Network() string { return "smux" }
func (muxAddr) String() string  { return "devnat" }

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
