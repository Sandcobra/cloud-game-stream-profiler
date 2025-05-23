package metrics

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Metric struct {
	Timestamp            time.Time `json:"timestamp"`
	FPS                  float64   `json:"fps"`
	Latency              float64   `json:"latency_ms"`
	Bitrate              float64   `json:"bitrate_kbps"`
	SessionID            string    `json:"session_id"`
	PredictedDegradation []string  `json:"predicted_degradation,omitempty"`
}

var (
	metricsMu      sync.Mutex
	sessionMetrics = make(map[string][]Metric)
)

func HandleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m Metric
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	m.Timestamp = time.Now()
	if m.SessionID == "" {
		m.SessionID = "session-" + time.Now().Format("150405")
	}
	metricsMu.Lock()
	sessionMetrics[m.SessionID] = append(sessionMetrics[m.SessionID], m)
	metricsMu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("webui/static/dashboard.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func HandleJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	session := r.URL.Query().Get("session")
	metricsMu.Lock()
	defer metricsMu.Unlock()
	if session == "" {
		json.NewEncoder(w).Encode([]Metric{})
		return
	}
	json.NewEncoder(w).Encode(sessionMetrics[session])
}

func HandleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	metricsMu.Lock()
	defer metricsMu.Unlock()
	sessions := make([]string, 0, len(sessionMetrics))
	for id := range sessionMetrics {
		sessions = append(sessions, id)
	}
	sort.Strings(sessions)
	json.NewEncoder(w).Encode(sessions)
}
