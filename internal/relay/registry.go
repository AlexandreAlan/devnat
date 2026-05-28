package relay

import (
	"sync"

	"github.com/xtaci/smux"
)

type tunnelEntry struct {
	subdomain string
	session   *smux.Session
}

// registry maps active subdomains to their agent multiplexer sessions.
type registry struct {
	mu      sync.RWMutex
	tunnels map[string]*tunnelEntry
}

func newRegistry() *registry {
	return &registry{tunnels: make(map[string]*tunnelEntry)}
}

// reserve claims a subdomain before the session is ready. It returns false if
// the subdomain is already taken.
func (r *registry) reserve(sub string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tunnels[sub]; exists {
		return false
	}
	r.tunnels[sub] = &tunnelEntry{subdomain: sub}
	return true
}

// bind attaches the live session to a previously reserved subdomain.
func (r *registry) bind(sub string, sess *smux.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tunnels[sub]; ok {
		t.session = sess
	}
}

func (r *registry) remove(sub string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tunnels, sub)
}

// session returns the live session for a subdomain, if one is bound.
func (r *registry) session(sub string) (*smux.Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tunnels[sub]
	if !ok || t.session == nil {
		return nil, false
	}
	return t.session, true
}

func (r *registry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tunnels)
}
