package agent

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"
)

//go:embed web/index.html
var dashFS embed.FS

type reqRecord struct {
	Time       time.Time `json:"time"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMs int64     `json:"durationMs"`
}

// inspector keeps a bounded, in-memory ring of recent requests for the
// local dashboard.
type inspector struct {
	mu      sync.RWMutex
	max     int
	records []reqRecord
}

func newInspector(max int) *inspector { return &inspector{max: max} }

func (i *inspector) record(r reqRecord) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.records = append(i.records, r)
	if len(i.records) > i.max {
		i.records = i.records[len(i.records)-i.max:]
	}
}

// snapshot returns recent requests, newest first.
func (i *inspector) snapshot() []reqRecord {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make([]reqRecord, len(i.records))
	copy(out, i.records)
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}

func (a *Agent) serveDashboard() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(a.insp.snapshot())
	})
	mux.HandleFunc("/api/meta", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"local": a.cfg.Local,
		})
	})
	if sub, err := fs.Sub(dashFS, "web"); err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}
	if err := http.ListenAndServe(a.cfg.Dashboard, mux); err != nil {
		log.Printf("dashboard stopped: %v", err)
	}
}
